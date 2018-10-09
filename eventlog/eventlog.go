package eventlog

import (
    "os"
    "github.com/sirupsen/logrus"
    logrus_syslog "github.com/sirupsen/logrus/hooks/syslog"
    "log/syslog"
    "github.com/getsentry/raven-go"
    "github.com/evalphobia/logrus_sentry"
)

type LogConfig struct {
    Enable bool `json:"enable,omitempty"`
    Target string `json:"target,omitempty"`
    Level string `json:"level,omitempty"`
    Path string `json:"path,omitempty"`
    Format string `json:"format,omitempty"`
    TimeFormat string `json:"time_format,omitempty"`
    Sentry SentryConfig `json:"sentry,omitempty"`
    Syslog SyslogConfig `json:"syslog,omitempty"`
}

type SentryConfig struct {
    Enable bool `json:"enable,omitempty"`
    DSN string `json:"dsn,omitempty"`
}

type SyslogConfig struct {
    Enable bool `json:"enable,omitempty"`
    Protocol string `json:"protocol,omitempty"`
    Address string `json:"address,omitempty"`
}

type EventLogger struct {
    config *LogConfig
    file   *os.File
    log    *logrus.Logger
    sentryClient *raven.Client
}

var Logger *EventLogger

func NewLogger(config *LogConfig) *EventLogger {
    logger := &EventLogger {
        config: config,
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
            logger.log.Formatter = &logrus.JSONFormatter{TimestampFormat:config.TimeFormat}
        case "text":
            logger.log.Formatter = &logrus.TextFormatter{TimestampFormat:config.TimeFormat}
        default:
            logger.log.Formatter = &logrus.TextFormatter{TimestampFormat:config.TimeFormat}
        }
        if config.Sentry.Enable {
            if client, err := raven.New(config.Sentry.DSN); err == nil {
                if hook, err := logrus_sentry.NewWithClientSentryHook(client, []logrus.Level{logrus.PanicLevel, logrus.FatalLevel, logrus.ErrorLevel}); err == nil {
                    logger.log.Hooks.Add(hook)
                } else {
                    logger.log.Errorf("cannot create sentry hook : %s", err)
                    logger.config.Sentry.Enable = false
                }
            } else {
                logger.log.Errorf("cannot create sentry client : %s", err)
                logger.config.Sentry.Enable = false
            }
        }
        if config.Syslog.Enable {
            priority := syslog.LOG_ERR
            switch logger.log.Level {
            case logrus.DebugLevel:
                priority = syslog.LOG_DEBUG
            case logrus.InfoLevel:
                priority = syslog.LOG_INFO
            case logrus.WarnLevel:
                priority = syslog.LOG_WARNING
            case logrus.ErrorLevel:
                priority = syslog.LOG_ERR
            default:
                priority = syslog.LOG_INFO
            }
            if hook, err := logrus_syslog.NewSyslogHook(config.Syslog.Protocol, config.Syslog.Address, priority, ""); err == nil {
                logger.log.Hooks.Add(hook)
            } else {
                logger.log.Errorf("cannot connect to syslog : %s : %s", config.Syslog.Address, err)
                config.Syslog.Enable = false
            }
        }
    }

    return logger
}

func (l *EventLogger) Log(fields map[string]interface{}, message string) {
    if l.config.Enable {
        l.log.WithFields(fields).Info(message)
    }
}

func (l *EventLogger) Debugf(format string, args ...interface{}) {
    if l.config.Enable {
        l.log.Debugf(format, args...)
    }
}

func (l *EventLogger) Debug(args ...interface{}) {
    if l.config.Enable {
        l.log.Debug(args...)
    }
}

func (l *EventLogger) Infof(format string, args ...interface{}) {
    if l.config.Enable {
        l.log.Infof(format, args...)
    }
}

func (l *EventLogger) Info(args ...interface{}) {
    if l.config.Enable {
        l.log.Info(args...)
    }
}

func (l *EventLogger) Warningf(format string, args ...interface{}) {
    if l.config.Enable {
        l.log.Warnf(format, args...)
    }
}

func (l *EventLogger) Warning(args ...interface{}) {
    if l.config.Enable {
        l.log.Warn(args...)
    }
}

func (l *EventLogger) Errorf(format string, args ...interface{}) {
    if l.config.Enable {
        l.log.Errorf(format, args...)
    }
}

func (l *EventLogger) Error(args ...interface{}) {
    if l.config.Enable {
        l.log.Error(args...)
    }
}

