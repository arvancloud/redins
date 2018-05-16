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

    "arvancloud/redins/redis"
    "arvancloud/redins/dns_types"
    "arvancloud/redins/eventlog"
    "github.com/patrickmn/go-cache"
    "arvancloud/redins/config"
)

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
    Enable         bool
    maxRequests    int
    updateInterval time.Duration
    checkInterval  time.Duration
    redisConfigServer *redis.Redis
    redisStatusServer *redis.Redis
    logger            *eventlog.EventLogger
    cachedItems       *cache.Cache
    lastUpdate        time.Time
}

func NewHealthcheck(config *config.RedinsConfig) *Healthcheck {
    h := &Healthcheck {
        Enable: config.HealthCheck.Enable,
        maxRequests: config.HealthCheck.MaxRequests,
        updateInterval: time.Duration(config.HealthCheck.UpdateInterval) * time.Second,
        checkInterval: time.Duration(config.HealthCheck.CheckInterval) * time.Second,
    }

    if h.Enable {

        h.redisConfigServer = redis.NewRedis(&config.Handler.Redis)
        h.redisStatusServer = redis.NewRedis(&config.HealthCheck.Redis)
        h.cachedItems = cache.New(time.Second * time.Duration(config.HealthCheck.CheckInterval), time.Duration(config.HealthCheck.CheckInterval) * time.Second * 10)
        h.transferItems()

        h.logger = eventlog.NewLogger(&config.HealthCheck.Log)
    }

    return h
}

func (h *Healthcheck) getStatus(host string, ip net.IP) int {
    if !h.Enable {
        return 0
    }
    key := host + ":" + ip.String()
    var item *HealthCheckItem
    val, found := h.cachedItems.Get(key)
    if !found {
        item = h.loadItem(key)
        if item == nil {
            item = new(HealthCheckItem)
        }
        h.cachedItems.Set(key, item, h.checkInterval)
    } else {
        item = val.(*HealthCheckItem)
    }
    return item.Status
}

func (h *Healthcheck) loadItem(key string) *HealthCheckItem {
    HostIp := strings.Split(key, ":")
    if len(HostIp) != 2 {
        log.Printf("[ERROR] invalid key: %s", key)
        return nil
    }
    item := new(HealthCheckItem)
    item.Host = HostIp[0]
    item.Ip = HostIp[1]
    itemStr := h.redisStatusServer.Get(key)
    if itemStr == "" {
        return nil
    }
    json.Unmarshal([]byte(itemStr), item)
    if item.DownCount > 0 {
        item.DownCount = -item.DownCount
    }
    return item
}

func (h *Healthcheck) storeItem(item *HealthCheckItem) {
    key := item.Host + ":" + item.Ip
    itemStr, err := json.Marshal(item)
    if err != nil {
        log.Printf("[ERROR] cannot marshal item to json : %s", err)
        return
    }
    h.redisStatusServer.Set(key, string(itemStr))
}

func (h *Healthcheck) transferItems() {
    itemsEqual := func(item1 *HealthCheckItem, item2 *HealthCheckItem) bool {
        if item1.Ip != item2.Ip || item1.Uri != item2.Uri || item1.Port != item2.Port ||
            item1.Protocol != item2.Protocol || item1.Enable != item2.Enable ||
            item1.UpCount != item2.UpCount || item1.DownCount != item2.DownCount || item1.Timeout != item2.Timeout {
            return false
        }
        return true
    }
    newItems := []*HealthCheckItem{}
    zones := h.redisConfigServer.GetKeys()
    for _, zone := range zones {
        subDomains := h.redisConfigServer.GetHKeys(zone)
        for _, subDomain := range subDomains {
            recordStr := h.redisConfigServer.HGet(zone, subDomain)
            record := new(dns_types.Record)
            record.Config.HealthCheckConfig.Enable = false
            err := json.Unmarshal([]byte(recordStr), record)
            if err != nil {
                log.Printf("[ERROR] cannot parse json : %s -> %s", recordStr, err)
                continue
            }
            var host string
            if subDomain == "@" {
                host = zone
            } else {
                host = subDomain + "." + zone
            }
            for _,ipRecord := range record.A {
                key := host + ":" + ipRecord.Ip.String()
                newItem := &HealthCheckItem {
                    Ip: ipRecord.Ip.String(),
                    Port: record.Config.HealthCheckConfig.Port,
                    Host: host,
                    Enable: record.Config.HealthCheckConfig.Enable,
                    DownCount: record.Config.HealthCheckConfig.DownCount,
                    UpCount: record.Config.HealthCheckConfig.UpCount,
                    Timeout: record.Config.HealthCheckConfig.Timeout,
                    Uri: record.Config.HealthCheckConfig.Uri,
                    Protocol: record.Config.HealthCheckConfig.Protocol,
                }
                oldItem := h.loadItem(key)
                if oldItem != nil && itemsEqual(oldItem, newItem) {
                    newItem.Status = oldItem.Status
                    newItem.LastCheck = oldItem.LastCheck
                }
                newItems = append(newItems, newItem)
            }
        }
    }
    h.redisStatusServer.Del("*")
    for _, item := range newItems {
        h.storeItem(item)
    }
}

func (h *Healthcheck) Start() {
    if h.Enable == false {
        return
    }
    wg := new(sync.WaitGroup)

    inputChan := make(chan *HealthCheckItem)

    for i := 0; i<h.maxRequests; i++ {
        wg.Add(1)
        go h.worker(inputChan, wg)
    }

    for {
        startTime := time.Now()
        keys := h.redisStatusServer.GetKeys()
        for _, key := range keys {
            item := h.loadItem(key)
            if item == nil || item.Enable == false {
                continue
            }
            if time.Since(item.LastCheck) > h.checkInterval {
                inputChan <- item
            }
        }
        if time.Since(h.lastUpdate) > h.updateInterval {
            wg.Wait()
            h.transferItems()
        }
        if time.Since(startTime) < h.checkInterval {
            time.Sleep(h.checkInterval - time.Since(startTime))
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
        if err != nil {
            log.Printf("[ERROR] invalid request : %s", err)
            h.statusDown(item)
        } else {
            req.Host = strings.TrimRight(item.Host, ".")
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
        h.storeItem(item)
        h.logHealthcheck(item)
    }
}

func (h *Healthcheck) logHealthcheck(item *HealthCheckItem) {
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

func (h *Healthcheck) FilterHealthcheck(qname string, record *dns_types.Record, ips []dns_types.IP_Record) []dns_types.IP_Record {
    newIps := []dns_types.IP_Record {}
    if h.Enable == false {
        newIps = append(newIps, ips...)
        return newIps
    }
    min := record.Config.HealthCheckConfig.DownCount
    for _, ip := range ips {
        status := h.getStatus(qname, ip.Ip)
        if  status > min {
            min = status
        }
    }
    // log.Println("[DEBUG] min = ", min)
    if min < record.Config.HealthCheckConfig.UpCount - 1 && min > record.Config.HealthCheckConfig.DownCount {
        min = record.Config.HealthCheckConfig.DownCount + 1
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
