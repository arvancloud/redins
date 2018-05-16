package healthcheck

import (
    "testing"
    "log"
    "net"
    "strings"
    "strconv"
    "time"
    "fmt"

    "arvancloud/redins/config"
    "arvancloud/redins/dns_types"
)

var healthcheckGetEntries = [][]string {
    {"w0.healthcheck.com.:1.2.3.4", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":3}"},
    {"w0.healthcheck.com.:2.3.4.5", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":1}"},
    {"w0.healthcheck.com.:3.4.5.6", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":0}"},
    {"w0.healthcheck.com.:4.5.6.7", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":-1}"},
    {"w0.healthcheck.com.:5.6.7.8", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":-3}"},

    {"w1.healthcheck.com.:2.3.4.5", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":1}"},
    {"w1.healthcheck.com.:3.4.5.6", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":0}"},
    {"w1.healthcheck.com.:4.5.6.7", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":-1}"},
    {"w1.healthcheck.com.:5.6.7.8", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":-3}"},

    {"w2.healthcheck.com.:3.4.5.6", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":0}"},
    {"w2.healthcheck.com.:4.5.6.7", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":-1}"},
    {"w2.healthcheck.com.:5.6.7.8", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":-3}"},

    {"w3.healthcheck.com.:4.5.6.7", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":-1}"},
    {"w3.healthcheck.com.:5.6.7.8", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":-3}"},

    {"w4.healthcheck.com.:5.6.7.8", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80, \"status\":-3}"},
}

var stats = []int { 3, 1, 0, -1, -3, 1, 0, -1, -3, 0, -1, -3, -1, -3, -3}
var filterResult = []int { 1, 3, 2, 1, 1}


var healthCheckSetEntries = [][]string {
    {"@", "185.143.233.2",
        "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80}",
    },
    {"www", "185.143.234.50",
        "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"\",\"port\":80}",
    },
}

var healthcheckTransferItems = [][]string{
    {"w0", "1.2.3.4",
        "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"uri0\",\"port\":80, \"status\":3}",
        "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"uri0\",\"port\":80, \"status\":2}",
    },
    {"w1", "2.3.4.5",
        "{\"enable\":false,\"protocol\":\"https\",\"uri\":\"uri111\",\"port\":8081}",
        "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"uri1\",\"port\":80, \"status\":3}",
    },
    {"w2", "3.4.5.6",
        "",
        "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"uri2\",\"port\":80, \"status\":3}",
    },
    {"w3", "4.5.6.7",
        "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"uri3\",\"port\":80, \"status\":3}",
        "",
    },
}

var healthCheckTransferResults = [][]string {
    {"w0.healthcheck.com.:1.2.3.4", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"uri0\",\"port\":80, \"status\":2}"},
    {"w1.healthcheck.com.:2.3.4.5", "{\"enable\":false,\"protocol\":\"https\",\"uri\":\"uri111\",\"port\":8081, \"status\":0}"},
    {"w3.healthcheck.com.:4.5.6.7", "{\"enable\":true,\"protocol\":\"http\",\"uri\":\"uri3\",\"port\":80, \"status\":0}"},
}

func TestGet(t *testing.T) {
    log.Println("TestGet")
    cfg := config.LoadConfig("config.json")
    h := NewHealthcheck(cfg)

    h.redisStatusServer.Del("*")
    for _, entry := range healthcheckGetEntries {
        h.redisStatusServer.Set(entry[0], entry[1])
    }

    for i,_ := range healthcheckGetEntries {
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
    cfg := config.LoadConfig("config.json")
    h := NewHealthcheck(cfg)

    for _, entry := range healthcheckGetEntries {
        h.redisStatusServer.Set(entry[0], entry[1])
    }

    w := []dns_types.Record {
        {
            Config: dns_types.RecordConfig {
                IpFilterMode: "multi",
                HealthCheckConfig: dns_types.HealthCheckRecordConfig {
                    Enable: true,
                    DownCount: -3,
                    UpCount: 3,
                    Timeout: 1000,
                },
            },
            RRSet: dns_types.RRSet {
                A: []dns_types.IP_Record{
                    {Ip: net.ParseIP("1.2.3.4")},
                    {Ip: net.ParseIP("2.3.4.5")},
                    {Ip: net.ParseIP("3.4.5.6")},
                    {Ip: net.ParseIP("4.5.6.7")},
                    {Ip: net.ParseIP("5.6.7.8")},
                },
            },
        },
        {
            Config: dns_types.RecordConfig {
                IpFilterMode: "multi",
                HealthCheckConfig: dns_types.HealthCheckRecordConfig {
                    Enable: true,
                    DownCount: -3,
                    UpCount: 3,
                    Timeout: 1000,
                },
            },
            RRSet: dns_types.RRSet {
                A: []dns_types.IP_Record{
                    {Ip: net.ParseIP("2.3.4.5")},
                    {Ip: net.ParseIP("3.4.5.6")},
                    {Ip: net.ParseIP("4.5.6.7")},
                    {Ip: net.ParseIP("5.6.7.8")},
                },
            },
        },
        {
            Config: dns_types.RecordConfig {
                IpFilterMode: "multi",
                HealthCheckConfig: dns_types.HealthCheckRecordConfig {
                    Enable: true,
                    DownCount: -3,
                    UpCount: 3,
                    Timeout: 1000,
                },
            },
            RRSet: dns_types.RRSet {
                A: []dns_types.IP_Record{
                    {Ip: net.ParseIP("3.4.5.6")},
                    {Ip: net.ParseIP("4.5.6.7")},
                    {Ip: net.ParseIP("5.6.7.8")},
                },
            },
        },
        {
            Config: dns_types.RecordConfig {
                IpFilterMode: "multi",
                HealthCheckConfig: dns_types.HealthCheckRecordConfig {
                    Enable: true,
                    DownCount: -3,
                    UpCount: 3,
                    Timeout: 1000,
                },
            },
            RRSet: dns_types.RRSet {
                A: []dns_types.IP_Record{
                    {Ip: net.ParseIP("4.5.6.7")},
                    {Ip: net.ParseIP("5.6.7.8")},
                },
            },
        },
        {
            Config: dns_types.RecordConfig {
                IpFilterMode: "multi",
                HealthCheckConfig: dns_types.HealthCheckRecordConfig {
                    Enable: true,
                    DownCount: -3,
                    UpCount: 3,
                    Timeout: 1000,
                },
            },
            RRSet: dns_types.RRSet {
                A: []dns_types.IP_Record{
                    {Ip: net.ParseIP("5.6.7.8")},
                },
            },
        },
    }
    for i, _ := range w {
        log.Println("[DEBUG]", w[i])
        w[i].A = h.FilterHealthcheck("w" + strconv.Itoa(i) + ".healthcheck.com.", &w[i], w[i].A)
        log.Println("[DEBUG]", w[i])
        if len(w[i].A) != filterResult[i] {
            t.Fail()
        }
    }
    h.redisStatusServer.Del("*")
    // h.Stop()
}

func TestSet(t *testing.T) {
    log.Println("TestSet")
    cfg := config.LoadConfig("config.json")
    h := NewHealthcheck(cfg)

    h.redisConfigServer.Del("*")
    h.redisStatusServer.Del("*")
    for _, str  := range healthCheckSetEntries {
        a := fmt.Sprintf("{\"a\":[{\"ttl\":300, \"ip\":\"%s\"}],\"config\":{\"ip_filter_mode\":\"multi\", \"health_check\":%s}}", str[1], str[2])
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
    cfg := config.LoadConfig("config.json")
    h := NewHealthcheck(cfg)

    h.redisConfigServer.Del("*")
    h.redisStatusServer.Del("*")
    for _, str  := range healthcheckTransferItems {
        if str[2] != "" {
            a := fmt.Sprintf("{\"a\":[{\"ttl\":300, \"ip\":\"%s\"}],\"config\":{\"ip_filter_mode\":\"multi\", \"health_check\":%s}}", str[1], str[2])
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
        if !itemsEqual(resItem, storedItem) {
            t.Fail()
        }
    }
}