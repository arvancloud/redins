package healthcheck

import (
    "crypto/tls"
    "time"
    "encoding/json"
    "strings"
    "log"
    "sync"
    "net"
    "net/http"

    "github.com/go-ini/ini"
    "arvancloud/redins/redis"
    "arvancloud/redins/handler"
    "arvancloud/redins/eventlog"
)

type HealthcheckConfig struct {
    enable         bool
    maxRequests    int
    updateInterval time.Duration
    checkInterval  time.Duration
    redisConfig    *redis.RedisConfig
    loggerConfig   *eventlog.LoggerConfig
}

type HealthCheckItem struct {
    Protocol  string        `json:"protocol,omitempty"`
    Uri       string        `json:"uri,omitempty"`
    Port      int           `json:"port,omitempty"`
    Status    int           `json:"status,omitempty"`
    LastCheck time.Time     `json:"lastcheck,omitempty"`
    Timeout   time.Duration `json:"timeout,omitempty"`
    UpCount   int           `json:"up_count,omitempty"`
    DownCount int           `json:"down_count,omitempty"`
    Enable    bool          `json:"enable,omitempty"`
    Host      string        `json:"-"`
    Ip        string        `json:"-"`
}

type Healthcheck struct {
    config      *HealthcheckConfig
    redisServer *redis.Redis
    logger      *eventlog.EventLogger
    items       map[string]*HealthCheckItem
    lastUpdate  time.Time
}

func LoadConfig(cfg *ini.File, section string) *HealthcheckConfig {
    healthcheckConfig := cfg.Section(section)
    redisSection := healthcheckConfig.Key("redis").MustString("redis")
    logSection := healthcheckConfig.Key("log").MustString("log")
    return &HealthcheckConfig {
        enable:         healthcheckConfig.Key("enable").MustBool(true),
        maxRequests:    healthcheckConfig.Key("max_requests").MustInt(20),
        updateInterval: healthcheckConfig.Key("update_interval").MustDuration(10 * time.Minute),
        checkInterval:  healthcheckConfig.Key("check_interval").MustDuration(10 * time.Second),
        redisConfig:    redis.LoadConfig(cfg, redisSection),
        loggerConfig:   eventlog.LoadConfig(cfg, logSection),
    }
}

func NewHealthcheck(config *HealthcheckConfig) *Healthcheck {
    h := &Healthcheck {
        config: config,
    }

    if h.config.enable {

        h.redisServer = redis.NewRedis(config.redisConfig)
        h.items = make(map[string]*HealthCheckItem)
        h.loadItems()

        h.logger = eventlog.NewLogger(h.config.loggerConfig)
    }

    return h
}

func (h *Healthcheck) getStatus(host string, ip net.IP) int {
    if !h.config.enable {
        return 0
    }
    key := host + ":" + ip.String()
    i, ok := h.items[key]
    if ok {
        return i.Status
    }
    return 0
}

func (h *Healthcheck) newItem(key string) *HealthCheckItem {
    HostIp := strings.Split(key, ":")
    if len(HostIp) != 2 {
        log.Printf("[ERROR] invalid key: %s", key)
        return nil
    }
    item := new(HealthCheckItem)
    item.Host = HostIp[0]
    item.Ip = HostIp[1]
    itemStr := h.redisServer.Get(key)
    json.Unmarshal([]byte(itemStr), item)
    if item.DownCount > 0 {
        item.DownCount = -item.DownCount
    }
    return item
}

func (h *Healthcheck) loadItems() {
    itemsEqual := func(item1 *HealthCheckItem, item2 *HealthCheckItem) bool {
        if item1.Ip != item2.Ip || item1.Host != item2.Host || item1.Uri != item2.Uri || item1.Port != item2.Port || item1.Protocol != item2.Protocol {
            return false
        }
        return true
    }
    newItems := make(map[string]*HealthCheckItem)
    keys := h.redisServer.GetKeys()
    for _, key := range keys {
        newItem := h.newItem(key)
        if newItem == nil || newItem.Enable == false {
            continue
        }
        i, ok := h.items[key]
        if ok && itemsEqual(newItem, i) {
            newItems[key] = i
        } else {
            newItems[key] = newItem
        }
    }
    h.items = newItems
    h.lastUpdate = time.Now()
}


func (h *Healthcheck) Start() {
    if h.config.enable == false {
        return
    }
    wg := new(sync.WaitGroup)

    inputChan := make(chan *HealthCheckItem)

    for i := 0; i<h.config.maxRequests; i++ {
        wg.Add(1)
        go h.worker(inputChan, wg)
    }

    for {
        startTime := time.Now()
        for _, item := range h.items {
            if time.Since(item.LastCheck) > h.config.checkInterval {
                inputChan <- item
            }
        }
        if time.Since(h.lastUpdate) > h.config.updateInterval {
            h.loadItems()
        }
        if time.Since(startTime) < h.config.checkInterval {
            time.Sleep(h.config.checkInterval - time.Since(startTime))
        }
    }
}

func (h *Healthcheck) worker(inputChan chan *HealthCheckItem, wg *sync.WaitGroup) {
    defer wg.Done()
    for item := range inputChan {
        client := &http.Client{
            Timeout: time.Duration(item.Timeout),
            Transport: &http.Transport{
                TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
            },
            CheckRedirect: func(req *http.Request, via []*http.Request) error {
                return http.ErrUseLastResponse
            },
        }
        url := item.Protocol + "://" + item.Ip + item.Uri
        // log.Println("[DEBUG]", item)
        req, err := http.NewRequest("HEAD", url, nil)
        req.Host = item.Host
        if err != nil {
            log.Printf("[ERROR] invalid request : %s", err)
            h.statusDown(item)
        } else {
            resp, err := client.Do(req)
            if err != nil {
                log.Printf("[ERROR] request failed : %s", err)
                h.statusDown(item)
            } else {
                // log.Printf("[INFO] http response : ", resp.Status)
                switch resp.StatusCode {
                case http.StatusOK, http.StatusFound, http.StatusMovedPermanently:
                    h.statusUp(item)
                default:
                    h.statusDown(item)
                }
            }
        }
        item.LastCheck = time.Now()
        h.logHealthcheck(item)
    }
}

func (h *Healthcheck) logHealthcheck(item *HealthCheckItem) {
    if h.config.loggerConfig.Enable == false {
        return
    }

    type HealthcheckLogData struct {
        Ip string
        Port int
        Host string
        Uri string
        Status int
    }

    data := HealthcheckLogData {
        Ip: item.Ip,
        Port: item.Port,
        Host: item.Host,
        Uri: item.Uri,
        Status: item.Status,
    }

    h.logger.Log(data,"healthcheck")
}

func (h *Healthcheck) statusDown(item *HealthCheckItem) {
    if item.Status <= 0 {
        item.Status--
        if item.Status < item.DownCount {
            item.Status = item.DownCount
        }
    } else {
        item.Status = -1
    }
}

func (h *Healthcheck) statusUp(item *HealthCheckItem) {
    if item.Status >= 0 {
        item.Status++
        if item.Status > item.UpCount {
            item.Status = item.UpCount
        }
    } else {
        item.Status = 1
    }
}

func (h *Healthcheck) FilterHealthcheck(qname string, record *handler.Record, ips []handler.IP_Record) []handler.IP_Record {
    newIps := []handler.IP_Record {}
    if h.config.enable == false {
        newIps = append(newIps, ips...)
        return newIps
    }
    min := record.ZoneCfg.HealthCheckConfig.DownCount
    for _, ip := range ips {
        status := h.getStatus(qname, ip.Ip)
        if  status > min {
            min = status
        }
    }
    // log.Println("[DEBUG] min = ", min)
    if min < record.ZoneCfg.HealthCheckConfig.UpCount - 1 && min > record.ZoneCfg.HealthCheckConfig.DownCount {
        min = record.ZoneCfg.HealthCheckConfig.DownCount + 1
    }
    // log.Println("[DEBUG] min = ", min)
    for _, ip := range ips {
        // log.Println("[DEBUG]", qname, ":", ip.Ip.String(), "status: ", h.getStatus(qname, ip.Ip))
        if h.getStatus(qname, ip.Ip) < min {
            continue
        }
        newIps = append(newIps, ip)
    }
    return newIps
}