package handler

import (
    "testing"
    "log"
    "net"
    "strings"
    "strconv"
    "time"
    "fmt"

    "arvancloud/redins/eventlog"
    "arvancloud/redins/redis"
)

var healthcheckGetEntries = [][]string {
    {"w0.healthcheck.com.:1.2.3.4", `{"enable":true,"protocol":"http","uri":"/","port":80, "status":3}`},
    {"w0.healthcheck.com.:2.3.4.5", `{"enable":true,"protocol":"http","uri":"/","port":80, "status":1}`},
    {"w0.healthcheck.com.:3.4.5.6", `{"enable":true,"protocol":"http","uri":"/","port":80, "status":0}`},
    {"w0.healthcheck.com.:4.5.6.7", `{"enable":true,"protocol":"http","uri":"/","port":80, "status":-1}`},
    {"w0.healthcheck.com.:5.6.7.8", `{"enable":true,"protocol":"http","uri":"/","port":80, "status":-3}`},

    {"w1.healthcheck.com.:2.3.4.5", `{"enable":true,"protocol":"http","uri":"/","port":80, "status":1}`},
    {"w1.healthcheck.com.:3.4.5.6", `{"enable":true,"protocol":"http","uri":"/","port":80, "status":0}`},
    {"w1.healthcheck.com.:4.5.6.7", `{"enable":true,"protocol":"http","uri":"/","port":80, "status":-1}`},
    {"w1.healthcheck.com.:5.6.7.8", `{"enable":true,"protocol":"http","uri":"/","port":80, "status":-3}`},

    {"w2.healthcheck.com.:3.4.5.6", `{"enable":true,"protocol":"http","uri":"/","port":80, "status":0}`},
    {"w2.healthcheck.com.:4.5.6.7", `{"enable":true,"protocol":"http","uri":"/","port":80, "status":-1}`},
    {"w2.healthcheck.com.:5.6.7.8", `{"enable":true,"protocol":"http","uri":"/","port":80, "status":-3}`},

    {"w3.healthcheck.com.:4.5.6.7", `{"enable":true,"protocol":"http","uri":"/","port":80, "status":-1}`},
    {"w3.healthcheck.com.:5.6.7.8", `{"enable":true,"protocol":"http","uri":"/","port":80, "status":-3}`},

    {"w4.healthcheck.com.:5.6.7.8", `{"enable":true,"protocol":"http","uri":"/","port":80, "status":-3}`},
}

var stats = []int { 3, 1, 0, -1, -3, 1, 0, -1, -3, 0, -1, -3, -1, -3, -3}
var filterResult = []int { 1, 3, 2, 1, 1}


var healthCheckSetEntries = [][]string {
    {"@", "185.143.233.2",
        `{"enable":true,"protocol":"http","uri":"","port":80, "timeout": 1000}`,
    },
    {"www", "185.143.234.50",
        `{"enable":true,"protocol":"http","uri":"","port":80, "timeout": 1000}`,
    },
}

var healthcheckTransferItems = [][]string{
    {"w0", "1.2.3.4",
        `{"enable":true,"protocol":"http","uri":"/uri0","port":80, "status":3, "up_count": 3, "down_count": -3, "timeout":1000}`,
        `{"enable":true,"protocol":"http","uri":"/uri0","port":80, "status":2, "up_count": 3, "down_count": -3, "timeout":1000}`,
    },
    {"w1", "2.3.4.5",
        `{"enable":false,"protocol":"https","uri":"/uri111","port":8081, "up_count": 3, "down_count": -3, "timeout":1000}`,
        `{"enable":true,"protocol":"http","uri":"/uri1","port":80, "status":3, "up_count": 3, "down_count": -3, "timeout":1000}`,
    },
    {"w2", "3.4.5.6",
        "",
        `{"enable":true,"protocol":"http","uri":"/uri2","port":80, "status":3, "up_count": 3, "down_count": -3, "timeout":1000}`,
    },
    {"w3", "4.5.6.7",
        `{"enable":true,"protocol":"http","uri":"/uri3","port":80, "status":3, "up_count": 3, "down_count": -3, "timeout":1000}`,
        ``,
    },
}

var healthCheckTransferResults = [][]string {
    {"w0.healthcheck.com.:1.2.3.4", `{"enable":true,"protocol":"http","uri":"/uri0","port":80, "status":2, "up_count": 3, "down_count": -3, "timeout":1000}`},
    {"w1.healthcheck.com.:2.3.4.5", `{"enable":false,"protocol":"https","uri":"/uri111","port":8081, "status":0, "up_count": 3, "down_count": -3, "timeout":1000}`},
    {"w3.healthcheck.com.:4.5.6.7", `{"enable":true,"protocol":"http","uri":"/uri3","port":80, "status":0, "up_count": 3, "down_count": -3, "timeout":1000}`},
}

var config = HealthcheckConfig {
    Enable: true,
    MaxRequests: 10,
    UpdateInterval: 600,
    CheckInterval: 600,
    RedisStatusServer: redis.RedisConfig {
        Ip: "redis",
        Port: 6379,
        Password: "",
        Prefix: "healthcheck_",
        Suffix: "_healthcheck",
        ConnectTimeout: 0,
        ReadTimeout: 0,
    },
    Log: eventlog.LogConfig {
        Enable: true,
        Path: "/tmp/healthcheck.log",
    },
}

func TestGet(t *testing.T) {
    log.Println("TestGet")
    eventlog.Logger = eventlog.NewLogger(&eventlog.LogConfig{})
    configRedis := redis.NewRedis(&config.RedisStatusServer)
    h := NewHealthcheck(&config, configRedis)

    h.redisStatusServer.Del("*")
    for _, entry := range healthcheckGetEntries {
        h.redisStatusServer.Set(entry[0], entry[1])
    }

    for i := range healthcheckGetEntries {
        hostIp := strings.Split(healthcheckGetEntries[i][0], ":")
        stat := h.getStatus(hostIp[0], net.ParseIP(hostIp[1]))
        log.Println("[DEBUG]", stat, " ", stats[i])
        if stat != stats[i] {
            t.Fail()
        }
    }
    // h.Stop()
    h.redisStatusServer.Del("*")
}

func TestFilter(t *testing.T) {
    log.Println("TestFilter")
    eventlog.Logger = eventlog.NewLogger(&eventlog.LogConfig{})
    configRedis := redis.NewRedis(&config.RedisStatusServer)
    h := NewHealthcheck(&config, configRedis)

    for _, entry := range healthcheckGetEntries {
        h.redisStatusServer.Set(entry[0], entry[1])
    }

    w := []Record {
        {
            RRSets: RRSets {
                A: IP_RRSet{
                    Data: []IP_RR {
                        {Ip: net.ParseIP("1.2.3.4")},
                        {Ip: net.ParseIP("2.3.4.5")},
                        {Ip: net.ParseIP("3.4.5.6")},
                        {Ip: net.ParseIP("4.5.6.7")},
                        {Ip: net.ParseIP("5.6.7.8")},
                    },
                    FilterConfig: IpFilterConfig {
                        Count: "multi",
                        Order: "none",
                        GeoFilter: "none",
                    },
                    HealthCheckConfig: IpHealthCheckConfig {
                        Enable: true,
                        DownCount: -3,
                        UpCount: 3,
                        Timeout: 1000,
                    },
                },
            },
        },
        {
            RRSets: RRSets {
                A: IP_RRSet{
                    Data: []IP_RR {
                        {Ip: net.ParseIP("2.3.4.5")},
                        {Ip: net.ParseIP("3.4.5.6")},
                        {Ip: net.ParseIP("4.5.6.7")},
                        {Ip: net.ParseIP("5.6.7.8")},
                    },
                    FilterConfig: IpFilterConfig {
                        Count: "multi",
                        Order: "none",
                        GeoFilter: "none",
                    },
                    HealthCheckConfig: IpHealthCheckConfig {
                        Enable: true,
                        DownCount: -3,
                        UpCount: 3,
                        Timeout: 1000,
                    },
                },
            },
        },
        {
            RRSets: RRSets {
                A: IP_RRSet{
                    Data: []IP_RR {
                        {Ip: net.ParseIP("3.4.5.6")},
                        {Ip: net.ParseIP("4.5.6.7")},
                        {Ip: net.ParseIP("5.6.7.8")},
                    },
                    FilterConfig: IpFilterConfig {
                        Count: "multi",
                        Order: "none",
                        GeoFilter: "none",
                    },
                    HealthCheckConfig: IpHealthCheckConfig {
                        Enable: true,
                        DownCount: -3,
                        UpCount: 3,
                        Timeout: 1000,
                    },
                },
            },
        },
        {
            RRSets: RRSets {
                A: IP_RRSet{
                    Data: []IP_RR {
                        {Ip: net.ParseIP("4.5.6.7")},
                        {Ip: net.ParseIP("5.6.7.8")},
                    },
                    FilterConfig: IpFilterConfig {
                        Count: "multi",
                        Order: "none",
                        GeoFilter: "none",
                    },
                    HealthCheckConfig: IpHealthCheckConfig {
                        Enable: true,
                        DownCount: -3,
                        UpCount: 3,
                        Timeout: 1000,
                    },
                },
            },
        },
        {
            RRSets: RRSets {
                A: IP_RRSet{
                    Data: []IP_RR {
                        {Ip: net.ParseIP("5.6.7.8")},
                    },
                    FilterConfig: IpFilterConfig {
                        Count: "multi",
                        Order: "none",
                        GeoFilter: "none",
                    },
                    HealthCheckConfig: IpHealthCheckConfig {
                        Enable: true,
                        DownCount: -3,
                        UpCount: 3,
                        Timeout: 1000,
                    },
                },
            },
        },
    }
    for i := range w {
        log.Println("[DEBUG]", w[i])
        ips := h.FilterHealthcheck("w" + strconv.Itoa(i) + ".healthcheck.com.", &w[i].A)
        log.Println("[DEBUG]", w[i])
        if len(ips) != filterResult[i] {
            t.Fail()
        }
    }
    h.redisStatusServer.Del("*")
    // h.Stop()
}

func TestSet(t *testing.T) {
    log.Println("TestSet")
    eventlog.Logger = eventlog.NewLogger(&eventlog.LogConfig{})
    configRedis := redis.NewRedis(&config.RedisStatusServer)
    h := NewHealthcheck(&config, configRedis)

    h.redisConfigServer.Del("*")
    h.redisStatusServer.Del("*")
    for _, str  := range healthCheckSetEntries {
        a := fmt.Sprintf("{\"a\":{\"ttl\":300, \"records\":[{\"ip\":\"%s\"}],\"health_check\":%s}}", str[1], str[2])
        h.redisConfigServer.HSet("healthcheck.com.", str[0], a)
        var key string
        if str[0] == "@" {
            key = fmt.Sprintf("arvancloud.com.:%s", str[1])
        } else {
            key = fmt.Sprintf("%s.arvancloud.com.:%s", str[0], str[1])
        }
        h.redisStatusServer.Set(key, str[2])
    }
    h.transferItems()
    go h.Start()
    time.Sleep(time.Second * 10)

    log.Println("[DEBUG]", h.getStatus("arvancloud.com", net.ParseIP("185.143.233.2")))
    log.Println("[DEBUG]", h.getStatus("www.arvancloud.com", net.ParseIP("185.143.234.50")))
}

func TestTransfer(t *testing.T) {
    log.Printf("TestTransfer")
    eventlog.Logger = eventlog.NewLogger(&eventlog.LogConfig{})
    configRedis := redis.NewRedis(&config.RedisStatusServer)
    h := NewHealthcheck(&config, configRedis)

    h.redisConfigServer.Del("*")
    h.redisStatusServer.Del("*")
    for _, str  := range healthcheckTransferItems {
        if str[2] != "" {
            a := fmt.Sprintf("{\"a\":{\"ttl\":300, \"records\":[{\"ip\":\"%s\"}],\"health_check\":%s}}", str[1], str[2])
            h.redisConfigServer.HSet("healthcheck.com.", str[0], a)
        }
        if str[3] != "" {
            key := fmt.Sprintf("%s.healthcheck.com.:%s", str[0], str[1])
            h.redisStatusServer.Set(key, str[3])
        }
    }

    h.transferItems()

    itemsEqual := func(item1 *HealthCheckItem, item2 *HealthCheckItem) bool {
        if item1.Ip != item2.Ip || item1.Uri != item2.Uri || item1.Port != item2.Port ||
            item1.Protocol != item2.Protocol || item1.Enable != item2.Enable ||
            item1.UpCount != item2.UpCount || item1.DownCount != item2.DownCount || item1.Timeout != item2.Timeout {
            return false
        }
        return true
    }

    for _, str := range healthCheckTransferResults {
        h.redisStatusServer.Set(str[0] + "res", str[1])
        resItem := h.loadItem(str[0] + "res")
        resItem.Ip = strings.TrimRight(resItem.Ip, "res")
        storedItem := h.loadItem(str[0])
        log.Println(resItem)
        log.Println(storedItem)
        if !itemsEqual(resItem, storedItem) {
            t.Fail()
        }
    }
}
