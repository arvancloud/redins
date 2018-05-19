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

type RequestLogData struct {
    SourceIP           string
    SourceCountry      string
    DestinationIp      string
    DestinationCountry string
    Record             string
    ClientSubnet       string
}

func NewLogger(config *config.LogConfig) *EventLogger {
    logger := &EventLogger {
        Enable: config.Enable,
        log:    logrus.New(),
    }
    if config.Enable {
        var err error
        logger.file, err = os.OpenFile(config.Path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
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
    if l.Enable {
        l.log.WithFields(logrus.Fields{message: data,}).Info(message)
    }
}