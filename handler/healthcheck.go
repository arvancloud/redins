package handler

import (
    "crypto/tls"
    "time"
    "encoding/json"
    "strings"
    "net"
    "net/http"

    "arvancloud/redins/redis"
    "arvancloud/redins/eventlog"
    "github.com/patrickmn/go-cache"
)

type HealthCheckItem struct {
    Protocol  string    `json:"protocol,omitempty"`
    Uri       string    `json:"uri,omitempty"`
    Port      int       `json:"port,omitempty"`
    Status    int       `json:"status,omitempty"`
    LastCheck time.Time `json:"lastcheck,omitempty"`
    Timeout   int       `json:"timeout,omitempty"`
    UpCount   int       `json:"up_count,omitempty"`
    DownCount int       `json:"down_count,omitempty"`
    Enable    bool      `json:"enable,omitempty"`
    DomainId  string    `json:"domain_uuid, omitempty"`
    Host      string    `json:"-"`
    Ip        string    `json:"-"`
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
                w.Client.Timeout = time.Duration(item.Timeout) * time.Millisecond
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

type HealthcheckConfig struct {
    Enable bool `json:"enable,omitempty"`
    MaxRequests int `json:"max_requests,omitempty"`
    UpdateInterval int `json:"update_interval,omitempty"`
    CheckInterval int `json:"check_interval,omitempty"`
    RedisStatusServer redis.RedisConfig `json:"redis,omitempty"`
    Log eventlog.LogConfig `json:"log,omitempty"`
}

func NewHealthcheck(config *HealthcheckConfig, redisConfigServer *redis.Redis) *Healthcheck {
    h := &Healthcheck {
        Enable: config.Enable,
        maxRequests: config.MaxRequests,
        updateInterval: time.Duration(config.UpdateInterval) * time.Second,
        checkInterval: time.Duration(config.CheckInterval) * time.Second,
    }

    if h.Enable {

        h.redisConfigServer = redisConfigServer
        h.redisStatusServer = redis.NewRedis(&config.RedisStatusServer)
        h.cachedItems = cache.New(time.Second * time.Duration(config.CheckInterval), time.Duration(config.CheckInterval) * time.Second * 10)
        h.transferItems()
        h.dispatcher = NewDispatcher(config.MaxRequests)
        h.logger = eventlog.NewLogger(&config.Log)
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
    splits := strings.SplitAfterN(key, ":", 2)
    // eventlog.Logger.Error(splits)
    if len(splits) != 2 {
        eventlog.Logger.Errorf("invalid key: %s", key)
        return nil
    }
    item := new(HealthCheckItem)
    item.Host = strings.TrimSuffix(splits[0], ":")
    item.Ip = splits[1]
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

func (h *Healthcheck) getDomainId(zone string) string {
    var cfg ZoneConfig
    val := h.redisConfigServer.HGet(zone, "@config")
    if len(val) > 0 {
        err := json.Unmarshal([]byte(val), &cfg)
        if err != nil {
            eventlog.Logger.Errorf("cannot parse zone config : %s", err)
        }
    }
    return cfg.DomainId
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
    var newItems []*HealthCheckItem
    zones := h.redisConfigServer.GetKeys("*")
    eventlog.Logger.Error(zones)
    for _, zone := range zones {
        domainId := h.getDomainId(zone)
        subDomains := h.redisConfigServer.GetHKeys(zone)
        for _, subDomain := range subDomains {
            if subDomain == "@config" {
                continue
            }
            recordStr := h.redisConfigServer.HGet(zone, subDomain)
            record := new(Record)
            record.A.HealthCheckConfig = IpHealthCheckConfig {
                Timeout: 1000,
                Port: 80,
                UpCount: 3,
                DownCount: -3,
                Protocol: "http",
                Uri: "/",
                Enable: false,
            }
            record.AAAA = record.A
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
            for _, rrset := range []*IP_RRSet{&record.A, &record.AAAA} {
                for i := range rrset.Data {
                    key := host + ":" + rrset.Data[i].Ip.String()
                    newItem := &HealthCheckItem{
                        Ip:        rrset.Data[i].Ip.String(),
                        Port:      rrset.HealthCheckConfig.Port,
                        Host:      host,
                        Enable:    rrset.HealthCheckConfig.Enable,
                        DownCount: rrset.HealthCheckConfig.DownCount,
                        UpCount:   rrset.HealthCheckConfig.UpCount,
                        Timeout:   rrset.HealthCheckConfig.Timeout,
                        Uri:       rrset.HealthCheckConfig.Uri,
                        Protocol:  rrset.HealthCheckConfig.Protocol,
                        DomainId:  domainId,
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
        keys := h.redisStatusServer.GetKeys("*")
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
        "ip":          item.Ip,
        "port":        item.Port,
        "domain_name": item.Host,
        "domain_uuid": item.DomainId,
        "uri":         item.Uri,
        "status":      item.Status,
    }

    h.logger.Log(data,"ar_dns_healthcheck")
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

func (h *Healthcheck) FilterHealthcheck(qname string, rrset *IP_RRSet) []IP_RR {
    var newIps []IP_RR
    if h.Enable == false {
        newIps = append(newIps, rrset.Data...)
        return newIps
    }
    min := rrset.HealthCheckConfig.DownCount
    for _, ip := range rrset.Data {
        status := h.getStatus(qname, ip.Ip)
        if  status > min {
            min = status
        }
    }
    eventlog.Logger.Debugf("min = %d", min)
    if min < rrset.HealthCheckConfig.UpCount - 1 && min > rrset.HealthCheckConfig.DownCount {
        min = rrset.HealthCheckConfig.DownCount + 1
    }
    eventlog.Logger.Debugf("min = %d", min)
    for _, ip := range rrset.Data {
        eventlog.Logger.Debug("qname: ", ip.Ip.String(), " status: ", h.getStatus(qname, ip.Ip))
        if h.getStatus(qname, ip.Ip) < min {
            continue
        }
        newIps = append(newIps, ip)
    }
    return newIps
}
