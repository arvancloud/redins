package healthcheck

import (
    "crypto/tls"
    "time"
    "encoding/json"
    "strings"
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
    Enable            bool
    maxRequests       int
    updateInterval    time.Duration
    checkInterval     time.Duration
    redisConfigServer *redis.Redis
    redisStatusServer *redis.Redis
    logger            *eventlog.EventLogger
    cachedItems       *cache.Cache
    lastUpdate        time.Time
    dispatcher        *Dispatcher
}

type Job *HealthCheckItem

type Dispatcher struct {
    WorkerPool chan chan Job
    MaxWorkers int
    JobQueue chan Job
}

func NewDispatcher(maxWorkers int) *Dispatcher {
    pool := make(chan chan Job, maxWorkers)
    return &Dispatcher {
        WorkerPool: pool,
        MaxWorkers: maxWorkers,
        JobQueue: make(chan Job, 100),
    }
}

func (d *Dispatcher) Run(healthcheck *Healthcheck) {
    for i := 0; i < d.MaxWorkers; i++ {
        worker := NewWorker(d.WorkerPool, healthcheck, i)
        worker.Start()
    }

    go d.dispatch()
}

func (d *Dispatcher) dispatch() {
    for {
        select {
        case job := <- d.JobQueue:
            go func(job Job) {
                jobChannel := <- d.WorkerPool
                jobChannel <- job
            }(job)
        }
    }
}

type Worker struct {
    Id int
    Client *http.Client
    healthcheck *Healthcheck
    WorkerPool chan chan Job
    JobChannel chan Job
    quit chan bool
}

func NewWorker(workerPool chan chan Job, healthcheck *Healthcheck, id int) Worker {
    tr := &http.Transport {
        MaxIdleConnsPerHost: 1024,
        TLSHandshakeTimeout: 0 * time.Second,
        TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
    }
    client := &http.Client {
        Transport: tr,
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            return http.ErrUseLastResponse
        },

    }

    return Worker {
        Id:          id,
        Client:      client,
        healthcheck: healthcheck,
        WorkerPool:  workerPool,
        JobChannel:  make(chan Job),
        quit:        make(chan bool),
    }
}

func (w Worker) Start() {
    go func() {
        eventlog.Logger.Debugf("Worker: worker %d started", w.Id)
        for {
            eventlog.Logger.Debugf("Worker: worker %d waiting for new job", w.Id)
            w.WorkerPool <- w.JobChannel
            select {
            case item := <- w.JobChannel:
                eventlog.Logger.Debugf("item %v received", item)
                w.Client.Timeout = time.Duration(item.Timeout)
                url := item.Protocol + "://" + item.Ip + item.Uri
                // log.Println("[DEBUG]", url)
                req, err := http.NewRequest("HEAD", url, nil)
                if err != nil {
                    eventlog.Logger.Errorf("invalid request : %s", err)
                    statusDown(item)
                } else {
                    req.Host = strings.TrimRight(item.Host, ".")
                    resp, err := w.Client.Do(req)
                    if err != nil {
                        eventlog.Logger.Errorf("request failed : %s", err)
                        statusDown(item)
                    } else {
                        // log.Printf("[INFO] http response : ", resp.Status)
                        switch resp.StatusCode {
                        case http.StatusOK, http.StatusFound, http.StatusMovedPermanently:
                            statusUp(item)
                        default:
                            statusDown(item)
                        }
                    }
                }
                item.LastCheck = time.Now()
                w.healthcheck.storeItem(item)
                w.healthcheck.logHealthcheck(item)

            case <- w.quit:
                return
            }
        }
    }()
}

func (w Worker) Stop() {
    go func() {
        w.quit <- true
    }()
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
        h.dispatcher = NewDispatcher(config.HealthCheck.MaxRequests)
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
        eventlog.Logger.Errorf("invalid key: %s", key)
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
        eventlog.Logger.Errorf("cannot marshal item to json : %s", err)
        return
    }
    eventlog.Logger.Debugf("setting %v in redis : %s", *item, string(itemStr))
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
            record := &dns_types.Record {
                Config: dns_types.RecordConfig {
                  HealthCheckConfig: dns_types.HealthCheckRecordConfig {
                    Timeout: 1000,
                    Port: 80,
                    UpCount: 3,
                    DownCount: -3,
                    Protocol: "http",
                    Uri: "/",
                    Enable: false,
                  },
                },
            }
            record.Config.HealthCheckConfig.Enable = false
            err := json.Unmarshal([]byte(recordStr), record)
            if err != nil {
                eventlog.Logger.Errorf("cannot parse json : %s -> %s", recordStr, err)
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
    h.dispatcher.Run(h)

    for {
        startTime := time.Now()
        keys := h.redisStatusServer.GetKeys()
        for _, key := range keys {
            item := h.loadItem(key)
            if item == nil || item.Enable == false {
                continue
            }
            if time.Since(item.LastCheck) > h.checkInterval {
                eventlog.Logger.Debugf("item %v added", item)
                h.dispatcher.JobQueue <- item
            }
        }
        if time.Since(h.lastUpdate) > h.updateInterval {
            h.transferItems()
        }
        if time.Since(startTime) < h.checkInterval {
            time.Sleep(h.checkInterval - time.Since(startTime))
        }
    }
}

func (h *Healthcheck) logHealthcheck(item *HealthCheckItem) {
    data := map[string]interface{} {
        "Ip":     item.Ip,
        "Port":   item.Port,
        "Host":   item.Host,
        "Uri":    item.Uri,
        "Status": item.Status,
    }

    h.logger.Log(data,"healthcheck")
}

func statusDown(item *HealthCheckItem) {
    if item.Status <= 0 {
        item.Status--
        if item.Status < item.DownCount {
            item.Status = item.DownCount
        }
    } else {
        item.Status = -1
    }
}

func statusUp(item *HealthCheckItem) {
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
    eventlog.Logger.Debugf("min = %d", min)
    if min < record.Config.HealthCheckConfig.UpCount - 1 && min > record.Config.HealthCheckConfig.DownCount {
        min = record.Config.HealthCheckConfig.DownCount + 1
    }
    eventlog.Logger.Debugf("min = %d", min)
    for _, ip := range ips {
        eventlog.Logger.Debugf("qname: %s, status: %d", ip.Ip.String(), h.getStatus(qname, ip.Ip))
        if h.getStatus(qname, ip.Ip) < min {
            continue
        }
        newIps = append(newIps, ip)
    }
    return newIps
}
