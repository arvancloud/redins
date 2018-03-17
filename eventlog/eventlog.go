package eventlog

import (
    "os"
    "github.com/sirupsen/logrus"
    "github.com/go-ini/ini"
)

type EventLogger struct {
    config *LoggerConfig
    file   *os.File
    log    *logrus.Logger
}

type LoggerConfig struct {
    Enable bool
    path   string
}

func LoadConfig(cfg *ini.File, section string) *LoggerConfig {
    logConfig := cfg.Section(section)
    return &LoggerConfig {
        Enable: logConfig.Key("enable").MustBool(true),
        path: logConfig.Key("path").MustString("/tmp/dns.log"),
    }
}

func NewLogger(config *LoggerConfig) *EventLogger {
    logger := &EventLogger {
        config: config,
        log:    logrus.New(),
    }
    if config.Enable {
        var err error
        logger.file, err = os.OpenFile(logger.config.path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
        if err == nil {
            logger.log.Out = logger.file
        } else {
            logger.log.Out = os.Stdout
            logger.log.Info("Failed to log to file, using default output ", err)
        }
        logger.log.Formatter = &logrus.JSONFormatter{}
    }
    return logger
}

func (l *EventLogger) Log(data interface{}, message string) {
    if l.config.Enable {
        l.log.WithFields(logrus.Fields{message: data,}).Info(message)
    }
}