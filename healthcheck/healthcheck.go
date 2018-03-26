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
    "github.com/hawell/redins/redis"
    "github.com/hawell/redins/handler"
    "github.com/hawell/redins/eventlog"
)

type HealthcheckConfig struct {
    enable         bool
    maxRequests    int
    updateInterval time.Duration
    checkInterval  time.Duration
    requestTimeout time.Duration
    upThreshold    int
    downThreshold  int
    redisConfig    *redis.RedisConfig
    loggerConfig   *eventlog.LoggerConfig
}

type HealthCheckItem struct {
    Protocol  string    `json:"protocol,omitempty"`
    Uri       string    `json:"uri,omitempty"`
    Port      int       `json:"port,omitempty"`
    Status    int       `json:"status,omitempty"`
    LastCheck time.Time `json:"lastcheck,omitempty"`
    Host      string    `json:"-"`
    Ip        string    `json:"-"`
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
        requestTimeout: healthcheckConfig.Key("request_timeout").MustDuration(10000 * time.Millisecond),
        upThreshold:    healthcheckConfig.Key("up_threshold").MustInt(3),
        downThreshold:  healthcheckConfig.Key("down_threshold").MustInt(-3),
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
        if newItem == nil {
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
            Timeout: time.Duration(h.config.requestTimeout),
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
        if item.Status < h.config.downThreshold {
            item.Status = h.config.upThreshold
        }
    } else {
        item.Status = -1
    }
}

func (h *Healthcheck) statusUp(item *HealthCheckItem) {
    if item.Status >= 0 {
        item.Status++
        if item.Status > h.config.upThreshold {
            item.Status = h.config.upThreshold
        }
    } else {
        item.Status = 1
    }
}

func (h *Healthcheck) FilterHealthcheck(qname string, record *handler.Record) {
    if h.config.enable == false {
        return
    }
    min := h.config.downThreshold
    for _, a := range record.A {
        status := h.getStatus(qname, a.Ip)
        if  status > min {
            min = status
        }
    }
    // log.Println("[DEBUG] min = ", min)
    if min < h.config.upThreshold - 1 && min > h.config.downThreshold {
        min = h.config.downThreshold + 1
    }
    // log.Println("[DEBUG] min = ", min)
    newA := []handler.A_Record {}
    for _, a := range record.A {
        // log.Println("[DEBUG]", qname, ":", a.Ip.String(), "status: ", h.getStatus(qname, a.Ip))
        if h.getStatus(qname, a.Ip) < min {
            continue
        }
        newA = append(newA, a)
    }
    record.A = newA
    // log.Println("[DEBUG]", newA)
    min = h.config.downThreshold
    for _, aaaa := range record.AAAA {
        status := h.getStatus(qname, aaaa.Ip)
        if  status > min {
            min = status
        }
    }
    if min < h.config.upThreshold && min > h.config.downThreshold {
        min = h.config.downThreshold + 1
    }
    newAAAA := []handler.AAAA_Record {}
    for _, aaaa := range record.AAAA {
        if h.getStatus(qname, aaaa.Ip) < min {
            continue
        }
        newAAAA = append(newAAAA, aaaa)
    }
    record.AAAA = newAAAA
}