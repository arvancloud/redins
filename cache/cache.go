package cache

import (
    "time"
    "container/heap"
    "github.com/hawell/redins/handler"
    "github.com/go-ini/ini"
)

type CacheItem struct {
    DnsRecord *handler.Record
    CreateTime time.Time
    AccessTime time.Time
    index int
    zone string
    location string
}

type DnsCache struct {
    config *CacheConfig
    records map[string]*CacheItem
    pq []string
}

type CacheConfig struct {
    enable bool
    size int
    timeout time.Duration
}

func LoadConfig(cfg *ini.File, section string) *CacheConfig {
    cacheConfig := cfg.Section(section)
    return &CacheConfig {
        enable: cacheConfig.Key("enable").MustBool(true),
        size: cacheConfig.Key("size").MustInt(10000),
        timeout: cacheConfig.Key("timeout").MustDuration(100 * time.Second),
    }
}

func NewCache(config *CacheConfig) *DnsCache {
    cache := &DnsCache {
        config: config,
        records: make(map[string]*CacheItem),
    }
    heap.Init(cache)
    return cache
}

func (c *DnsCache) Len() int {
    return len(c.pq)
}

func (c *DnsCache) Less(i, j int) bool {
    return c.records[c.pq[i]].AccessTime.Before(c.records[c.pq[j]].AccessTime)
}

func (c *DnsCache) Swap(i, j int) {
    c.pq[i], c.pq[j] = c.pq[j], c.pq[i]
    c.records[c.pq[i]].index = i
    c.records[c.pq[j]].index = j
}

func (c *DnsCache) Push(x interface{}) {
    n := len(c.pq)
    item := x.(*CacheItem)
    item.index = n
    c.pq = append(c.pq, item.location)
    c.records[item.location] = item
}

func (c *DnsCache) Pop() interface{} {
    n := len(c.pq)
    item := c.records[c.pq[n-1]]
    c.pq = c.pq[0 : n-1]
    delete(c.records, item.location)
    return item
}

func (c *DnsCache) Get(location string) (string, *handler.Record) {
    if !c.config.enable {
        return "", nil
    }
    i, ok := c.records[location]
    if !ok {
        return "", nil
    }
    if time.Since(i.CreateTime) > c.config.timeout {
        heap.Remove(c, i.index)
        delete(c.records, location)
        return "", nil
    }
    i.AccessTime = time.Now()
    heap.Fix(c, i.index)
    return i.zone, i.DnsRecord
}

func (c *DnsCache) Set(zone string, location string, record *handler.Record) {
    if !c.config.enable {
        return
    }
    if c.Len() >= c.config.size {
        for i := 0; i< c.Len()/10; i++ {
            heap.Pop(c)
        }
    }
    i, ok := c.records[location]
    if ok {
        i.DnsRecord = record
        i.zone = zone
        i.AccessTime = time.Now()
        heap.Fix(c, i.index)
    } else {
        item := &CacheItem {
            DnsRecord: record,
            AccessTime: time.Now(),
            CreateTime: time.Now(),
            location: location,
            zone: zone,
        }
        heap.Push(c, item)
    }
}