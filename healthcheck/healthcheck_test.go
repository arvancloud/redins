package healthcheck

import (
    "testing"
    "log"
    "net"
    "strings"
    "strconv"
    "time"

    "arvancloud/redins/handler"
    "github.com/go-ini/ini"
)

var healthcheckGetEntries = [][]string {
    {"w0.healthcheck.com:1.2.3.4", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":3}"},
    {"w0.healthcheck.com:2.3.4.5", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":1}"},
    {"w0.healthcheck.com:3.4.5.6", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":0}"},
    {"w0.healthcheck.com:4.5.6.7", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":-1}"},
    {"w0.healthcheck.com:5.6.7.8", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":-3}"},

    {"w1.healthcheck.com:2.3.4.5", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":1}"},
    {"w1.healthcheck.com:3.4.5.6", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":0}"},
    {"w1.healthcheck.com:4.5.6.7", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":-1}"},
    {"w1.healthcheck.com:5.6.7.8", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":-3}"},

    {"w2.healthcheck.com:3.4.5.6", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":0}"},
    {"w2.healthcheck.com:4.5.6.7", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":-1}"},
    {"w2.healthcheck.com:5.6.7.8", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":-3}"},

    {"w3.healthcheck.com:4.5.6.7", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":-1}"},
    {"w3.healthcheck.com:5.6.7.8", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":-3}"},

    {"w4.healthcheck.com:5.6.7.8", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":-3}"},
}

var stats = []int { 3, 1, 0, -1, -3, 1, 0, -1, -3, 0, -1, -3, -1, -3, -3}
var filterResult = []int { 1, 3, 2, 1, 1}


var healthCheckSetEntries = [][]string {
    {"arvancloud.com:185.143.233.2", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80}"},
    {"www.arvancloud.com:185.143.234.50", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80}"},
}

func TestGet(t *testing.T) {
    cfg, err := ini.LooseLoad("test.ini")
    if err != nil {
        log.Printf("[ERROR] loading config failed : %s", err)
        t.Fail()
    }
    h := NewHealthcheck(LoadConfig(cfg, "test"))

    for _, entry := range healthcheckGetEntries {
        h.redisServer.Set(entry[0], entry[1])
    }
    h.loadItems()

    for i,_ := range healthcheckGetEntries {
        hostIp := strings.Split(healthcheckGetEntries[i][0], ":")
        stat := h.getStatus(hostIp[0], net.ParseIP(hostIp[1]))
        log.Println("[DEBUG]", stat, " ", stats[i])
        if stat != stats[i] {
            t.Fail()
        }
    }
    // h.Stop()
    h.redisServer.Del("*")
}

func TestFilter(t *testing.T) {
    cfg, err := ini.LooseLoad("test.ini")
    if err != nil {
        log.Printf("[ERROR] loading config failed : %s", err)
        t.Fail()
    }
    h := NewHealthcheck(LoadConfig(cfg, "test"))

    for _, entry := range healthcheckGetEntries {
        h.redisServer.Set(entry[0], entry[1])
    }
    h.loadItems()

    w := []handler.Record {
        {
            ZoneCfg: handler.ZoneConfig {
              HealthCheckConfig: handler.HealthCheckConfig {
                Enable: true,
                DownCount: -3,
                UpCount: 3,
                RequestTimeout: 1000,
              },
            },
            Config: handler.RecordConfig { IpFilterMode: "multi" },
            A: []handler.IP_Record{
                {Ip: net.ParseIP("1.2.3.4")},
                {Ip: net.ParseIP("2.3.4.5")},
                {Ip: net.ParseIP("3.4.5.6")},
                {Ip: net.ParseIP("4.5.6.7")},
                {Ip: net.ParseIP("5.6.7.8")},
            },
        },
        {
            ZoneCfg: handler.ZoneConfig {
                HealthCheckConfig: handler.HealthCheckConfig {
                    Enable: true,
                    DownCount: -3,
                    UpCount: 3,
                    RequestTimeout: 1000,
                },
            },
            Config: handler.RecordConfig { IpFilterMode: "multi" },
            A: []handler.IP_Record {
                {Ip:net.ParseIP("2.3.4.5")},
                {Ip:net.ParseIP("3.4.5.6")},
                {Ip:net.ParseIP("4.5.6.7")},
                {Ip:net.ParseIP("5.6.7.8")},
            },
        },
        {
            ZoneCfg: handler.ZoneConfig {
                HealthCheckConfig: handler.HealthCheckConfig {
                    Enable: true,
                    DownCount: -3,
                    UpCount: 3,
                    RequestTimeout: 1000,
                },
            },
            Config: handler.RecordConfig { IpFilterMode: "multi" },
            A: []handler.IP_Record{
                {Ip: net.ParseIP("3.4.5.6")},
                {Ip: net.ParseIP("4.5.6.7")},
                {Ip: net.ParseIP("5.6.7.8")},
            },
        },
        {
            ZoneCfg: handler.ZoneConfig {
                HealthCheckConfig: handler.HealthCheckConfig {
                    Enable: true,
                    DownCount: -3,
                    UpCount: 3,
                    RequestTimeout: 1000,
                },
            },
            Config: handler.RecordConfig { IpFilterMode: "multi" },
            A: []handler.IP_Record{
                {Ip: net.ParseIP("4.5.6.7")},
                {Ip: net.ParseIP("5.6.7.8")},
            },
        },
        {
            ZoneCfg: handler.ZoneConfig {
                HealthCheckConfig: handler.HealthCheckConfig {
                    Enable: true,
                    DownCount: -3,
                    UpCount: 3,
                    RequestTimeout: 1000,
                },
            },
            Config: handler.RecordConfig { IpFilterMode: "multi" },
            A: []handler.IP_Record{
                {Ip: net.ParseIP("5.6.7.8")},
            },
        },
    }
    for i, _ := range w {
        log.Println("[DEBUG]", w[i])
        w[i].A = h.FilterHealthcheck("w" + strconv.Itoa(i) + ".healthcheck.com", &w[i], w[i].A)
        log.Println("[DEBUG]", w[i])
        if len(w[i].A) != filterResult[i] {
            t.Fail()
        }
    }
    h.redisServer.Del("*")
    // h.Stop()
}

func TestSet(t *testing.T) {
    cfg, err := ini.LooseLoad("test.ini")
    if err != nil {
        log.Printf("[ERROR] loading config failed : %s", err)
        t.Fail()
    }
    h := NewHealthcheck(LoadConfig(cfg, "test"))

    for _, entry := range healthCheckSetEntries {
        h.redisServer.Set(entry[0], entry[1])
    }
    h.loadItems()
    go h.Start()
    time.Sleep(time.Second * 10)

    log.Println("[DEBUG", h.getStatus("arvancloud.com", net.ParseIP("185.143.233.2")))
    log.Println("[DEBUG", h.getStatus("www.arvancloud.com", net.ParseIP("185.143.234.50")))
}