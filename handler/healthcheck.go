package handler

import (
    "fmt"
    "crypto/tls"
    "time"
    "encoding/json"
    "strings"
    "net"
    "net/http"
    "sync"

    "github.com/hawell/logger"
    "github.com/hawell/uperdis"
    "github.com/hawell/workerpool"
    "github.com/patrickmn/go-cache"
    "github.com/pkg/errors"
    "golang.org/x/net/icmp"
    "golang.org/x/net/ipv4"
    "encoding/binary"
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
    Error     error     `json:"-"`
}

type Healthcheck struct {
    Enable             bool
    maxRequests        int
    maxPendingRequests int
    updateInterval     time.Duration
    checkInterval      time.Duration
    redisConfigServer  *uperdis.Redis
    redisStatusServer  *uperdis.Redis
    logger             *logger.EventLogger
    cachedItems        *cache.Cache
    lastUpdate         time.Time
    dispatcher         *workerpool.Dispatcher
    quit               chan struct{}
    quitWG             sync.WaitGroup
}

func HandleHealthCheck(h *Healthcheck) workerpool.JobHandler {
    return func(worker *workerpool.Worker, job workerpool.Job) {
        item := job.(*HealthCheckItem)
        logger.Default.Debugf("item %v received", item)
        var err error = nil
        switch item.Protocol {
        case "http", "https":
            timeout := time.Duration(item.Timeout) * time.Millisecond
            url := item.Protocol + "://" + item.Ip + item.Uri
            err = httpCheck(url, item.Host, timeout)
        case "ping", "icmp":
            err = pingCheck(item.Ip, time.Duration(item.Timeout) * time.Millisecond)
            logger.Default.Error("@@@@@@@@@@@@@@ ", item.Ip, " : result : ", err)
        default:
            err = errors.New(fmt.Sprintf("invalid protocol : %s used for %s:%d", item.Protocol, item.Ip, item.Port))
            logger.Default.Error(err)
        }
        item.Error = err
        if err == nil {
            statusUp(item)
        } else {
            statusDown(item)
        }
        item.LastCheck = time.Now()
        h.storeItem(item)
        h.logHealthcheck(item)
    }
}

func httpCheck(url string,host string, timeout time.Duration) error {
    tr := &http.Transport {
        MaxIdleConnsPerHost: 1024,
        TLSHandshakeTimeout: 0 * time.Second,
        TLSClientConfig: &tls.Config {
            InsecureSkipVerify: true,
            ServerName: host,
        },
    }
    client := &http.Client {
        Transport: tr,
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            return http.ErrUseLastResponse
        },

    }
    req, err := http.NewRequest("HEAD", url, nil)
    if err != nil {
        logger.Default.Errorf("invalid request : %s", err)
        return err
    }
    req.Host = strings.TrimRight(host, ".")
    resp, err := client.Do(req)
    if err != nil {
        logger.Default.Errorf("request failed : %s", err)
        return err
    }
    switch resp.StatusCode {
    case http.StatusOK, http.StatusFound, http.StatusMovedPermanently:
        return nil
    default:
        return errors.New(fmt.Sprintf("invalid http status code : %d", resp.StatusCode))
    }
}

// FIXME: ping check is not working properly
func pingCheck(ip string, timeout time.Duration) error {
    c, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
    c.SetDeadline(time.Now().Add(timeout))
    if err != nil {
        return err
                }
    defer c.Close()

    id := int(binary.BigEndian.Uint32(net.ParseIP(ip)))
    wm := icmp.Message{
        Type: ipv4.ICMPTypeEcho, Code: 0,
        Body: &icmp.Echo{
            ID: id,
            Data: []byte("HELLO-R-U-THERE"),
        },
            }
    wb, err := wm.Marshal(nil)
    if err != nil {
        return err
        }
    if _, err := c.WriteTo(wb, &net.IPAddr{IP: net.ParseIP(ip)}); err != nil {
        return err
    }

    rb := make([]byte, 1500)
    n, _, err := c.ReadFrom(rb)
    if err != nil {
        return err
    }
    rm, err := icmp.ParseMessage(ipv4.ICMPTypeEchoReply.Protocol(), rb[:n])
    if err != nil {
        return err
    }
    switch rm.Type {
    case ipv4.ICMPTypeEchoReply:
        logger.Default.Error("@@@@@@@@@@@@ code = ", rm.Code)
        return nil
    default:
        return errors.New(fmt.Sprintf("got %+v; want echo reply", rm))
    }
}

type HealthcheckConfig struct {
    Enable bool `json:"enable,omitempty"`
    MaxRequests int `json:"max_requests,omitempty"`
    MaxPendingRequests int `json:"max_pending_requests,omitempty"`
    UpdateInterval int `json:"update_interval,omitempty"`
    CheckInterval int `json:"check_interval,omitempty"`
    RedisStatusServer uperdis.RedisConfig `json:"redis,omitempty"`
    Log logger.LogConfig `json:"log,omitempty"`
}

func NewHealthcheck(config *HealthcheckConfig, redisConfigServer *uperdis.Redis) *Healthcheck {
    h := &Healthcheck {
        Enable: config.Enable,
        maxRequests: config.MaxRequests,
        maxPendingRequests: config.MaxPendingRequests,
        updateInterval: time.Duration(config.UpdateInterval) * time.Second,
        checkInterval: time.Duration(config.CheckInterval) * time.Second,
    }

    if h.Enable {

        h.redisConfigServer = redisConfigServer
        h.redisStatusServer = uperdis.NewRedis(&config.RedisStatusServer)
        h.cachedItems = cache.New(time.Second * time.Duration(config.CheckInterval), time.Duration(config.CheckInterval) * time.Second * 10)
        h.transferItems()
        h.dispatcher = workerpool.NewDispatcher(config.MaxPendingRequests, config.MaxRequests)
        for i := 0; i < config.MaxRequests; i++ {
            h.dispatcher.AddWorker(HandleHealthCheck(h))
        }
        h.logger = logger.NewLogger(&config.Log)
	    h.quit = make(chan struct{}, 1)
    }

    return h
}

func (h *Healthcheck) ShutDown() {
    if !h.Enable {
        return
    }
    // fmt.Println("healthcheck : stopping")
    h.dispatcher.Stop()
    h.quitWG.Add(1)
    close(h.quit)
    h.quitWG.Wait()
    // fmt.Println("healthcheck : stopped")
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
    // logger.Default.Error(splits)
    if len(splits) != 2 {
        logger.Default.Errorf("invalid key: %s", key)
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
        logger.Default.Errorf("cannot marshal item to json : %s", err)
        return
    }
    logger.Default.Debugf("setting %v in redis : %s", *item, string(itemStr))
    h.redisStatusServer.Set(key, string(itemStr))
}

func (h *Healthcheck) getDomainId(zone string) string {
    var cfg ZoneConfig
    val := h.redisConfigServer.HGet(zone, "@config")
    if len(val) > 0 {
        err := json.Unmarshal([]byte(val), &cfg)
        if err != nil {
            logger.Default.Errorf("cannot parse zone config : %s", err)
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
                logger.Default.Errorf("cannot parse json : zone -> %s, %s -> %s", zone, recordStr, err)
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
    h.dispatcher.Run()

    for {
        select {
        case <-h.quit:
            // fmt.Println("healthcheck : quit")
            h.quitWG.Done()
            return
        case <-time.After(h.checkInterval):
        keys := h.redisStatusServer.GetKeys("*")
        for _, key := range keys {
            item := h.loadItem(key)
            if item == nil || item.Enable == false {
                continue
            }
            if time.Since(item.LastCheck) > h.checkInterval {
                logger.Default.Debugf("item %v added", item)
                h.dispatcher.JobQueue <- item
            }
        }
        case <-time.After(h.updateInterval):
            h.transferItems()
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
    if item.Error == nil {
        data["error"] = ""
    } else {
        data["error"] = item.Error.Error()
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
    logger.Default.Debugf("min = %d", min)
    if min < rrset.HealthCheckConfig.UpCount - 1 && min > rrset.HealthCheckConfig.DownCount {
        min = rrset.HealthCheckConfig.DownCount + 1
    }
    logger.Default.Debugf("min = %d", min)
    for _, ip := range rrset.Data {
        logger.Default.Debug("qname: ", ip.Ip.String(), " status: ", h.getStatus(qname, ip.Ip))
        if h.getStatus(qname, ip.Ip) < min {
            continue
        }
        newIps = append(newIps, ip)
    }
    return newIps
}
