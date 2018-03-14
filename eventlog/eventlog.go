package eventlog

import (
    "os"
    "github.com/sirupsen/logrus"
    "github.com/go-ini/ini"
    "github.com/coredns/coredns/request"
)

type EventLogger struct {
    config *LoggerConfig
    file   *os.File
}

type LoggerConfig struct {
    enable bool
    path   string
}

func LoadConfig(cfg *ini.File, section string) *LoggerConfig {
    logConfig := cfg.Section(section)
    return &LoggerConfig {
        enable: logConfig.Key("enable").MustBool(true),
        path: logConfig.Key("file").MustString("/tmp/dns.log"),
    }
}

func NewLogger(config *LoggerConfig) *EventLogger {
    logger := &EventLogger {
        config: config,
    }
    if config.enable {
        var err error
        logger.file, err = os.OpenFile(logger.config.path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
        if err == nil {
            logrus.SetOutput(logger.file)
        } else {
            logrus.Info("Failed to log to file, using default output ", err)
        }
        logrus.SetFormatter(&logrus.JSONFormatter{})
    }
    return logger
}

func (l *EventLogger) LogRequest(request *request.Request) {
    if !l.config.enable {
        return
    }

    type RequestLogData struct {
        SourceIP   		        string
        Record 			        string
        ClientSubnet           string
    }

    data := RequestLogData{
        SourceIP:         request.IP(),
        Record:           request.Name(),
    }

    opt := request.Req.IsEdns0()
    if len(opt.Option) != 0 {
        data.ClientSubnet= opt.Option[0].String()
    }

    logrus.WithFields(logrus.Fields{"data": data,}).Info("dns.request")
}
