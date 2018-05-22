package eventlog

import (
    "os"
    "github.com/sirupsen/logrus"
    "arvancloud/redins/config"
)

type EventLogger struct {
    Enable bool
    file   *os.File
    log    *logrus.Logger
}

var Logger *EventLogger

func NewLogger(config *config.LogConfig) *EventLogger {
    logger := &EventLogger {
        Enable: config.Enable,
        log:    logrus.New(),
    }
    if config.Enable {
        switch config.Level {
        case "debug":
            logger.log.Level = logrus.DebugLevel
        case "info":
            logger.log.Level = logrus.InfoLevel
        case "warning":
            logger.log.Level = logrus.WarnLevel
        case "error":
            logger.log.Level = logrus.ErrorLevel
        default:
            logger.log.Level = logrus.InfoLevel
        }
        switch config.Target {
        case "file":
            var err error
            logger.file, err = os.OpenFile(config.Path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
            if err == nil {
                logger.log.Out = logger.file
            } else {
                logger.log.Out = os.Stdout
                logger.log.Info("Failed to log to file, using default output ", err)
            }
        case "stdout":
            logger.log.Out = os.Stdout
        case "stderr":
            logger.log.Out = os.Stderr
        default:
            logger.log.Out = os.Stdout
        }
        switch config.Format {
        case "json":
            logger.log.Formatter = &logrus.JSONFormatter{}
        case "text":
            logger.log.Formatter = &logrus.TextFormatter{}
        default:
            logger.log.Formatter = &logrus.TextFormatter{}
        }
    }

    return logger
}

func (l *EventLogger) Log(fields map[string]interface{}, message string) {
    if l.Enable {
        l.log.WithFields(fields).Info(message)
    }
}

func (l *EventLogger) Debugf(format string, args ...interface{}) {
    if l.Enable {
        l.log.Debugf(format, args)
    }
}

func (l *EventLogger) Debug(args ...interface{}) {
    if l.Enable {
        l.log.Debug(args)
    }
}

func (l *EventLogger) Infof(format string, args ...interface{}) {
    if l.Enable {
        l.log.Infof(format, args)
    }
}

func (l *EventLogger) Info(args ...interface{}) {
    if l.Enable {
        l.log.Info(args)
    }
}

func (l *EventLogger) Warningf(format string, args ...interface{}) {
    if l.Enable {
        l.log.Warnf(format, args)
    }
}

func (l *EventLogger) Warning(args ...interface{}) {
    if l.Enable {
        l.log.Warn(args)
    }
}

func (l *EventLogger) Errorf(format string, args ...interface{}) {
    if l.Enable {
        l.log.Errorf(format, args)
    }
}

func (l *EventLogger) Error(args ...interface{}) {
    if l.Enable {
        l.log.Error(args)
    }
}

