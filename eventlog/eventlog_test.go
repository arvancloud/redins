package eventlog

import (
    "testing"
    "github.com/sirupsen/logrus"
    "os"
    "sync"
    "arvancloud/redins/config"
)



func TestAsyncLog(t *testing.T) {
    file1, _ := os.OpenFile("x.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)

    log1 := logrus.New()
    log1.Out = file1

    log2 := logrus.New()
    log2.Out = file1

    log1.Println("log1")
    log2.Println("log2")

    var wg sync.WaitGroup
    wg.Add(2)

    run := func (logger *logrus.Logger, m string) {
        for i := 0; i<100; i++ {
            logger.Println(m)
        }
        wg.Done()
    }

    go run(log1, "log1")
    go run(log2, "log2")
    wg.Wait()
}

func TestSentry(t *testing.T) {
    cfg := &config.LogConfig {
        Enable: true,
        Level: "debug",
        Target: "stdout",
        Format: "json",
        Sentry: config.SentryConfig {
            Enable: true,
            DSN: "https://7275419def7e4730aef88bd2c87d1ee7:860a3fd7794a48ee8232c1fb08f66b8e@error-tracking.arvancloud.com/4",
        },
    }

    logger := NewLogger(cfg)
    logger.Errorf("error error")
}