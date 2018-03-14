package cache

import (
    "testing"
    "time"
    "net"
    "fmt"
    "container/heap"
    "github.com/hawell/redins/handler"
)

func assertEqual(t *testing.T, a interface{}, b interface{}) {
    if !(a == b) {
        fmt.Printf("assertion failed : %v != %v\n", a, b)
        t.Fail()
    }
}

func TestCache(t *testing.T) {
    c := &DnsCache {
        config: &CacheConfig {
            enable: true,
            size: 100,
            timeout: time.Second * 100,
        },
        records: make(map[string]*CacheItem),
    }
    heap.Init(c)

    for i := 1; i <= 10; i++ {
        ip := fmt.Sprintf("%d.%d.%d.%d", i, i, i, i)
        location := ip
        item := CacheItem{
            location:   location,
            CreateTime: time.Now().Add(-(time.Second * time.Duration(i))),
            AccessTime: time.Now().Add(-(time.Second * time.Duration(i))),
            DnsRecord: &handler.Record{
                A: []handler.A_Record{
                    {Ip: net.ParseIP(ip)},
                },
            },
        }
        heap.Push(c, &item)
    }
    item := heap.Pop(c).(*CacheItem)
    assertEqual(t, item.location, "10.10.10.10")

    _, record := c.Get("9.9.9.9")
    assertEqual(t, record.A[0].Ip.String(), "9.9.9.9")
    item = heap.Pop(c).(*CacheItem)
    assertEqual(t, item.location, "8.8.8.8")
}


func TestCacheTimeout(t *testing.T) {
    c := &DnsCache {
        config: &CacheConfig{
            enable:  true,
            size:    100,
            timeout: time.Second * 1,
        },
        records: make(map[string]*CacheItem),
    }
    heap.Init(c)

    item := CacheItem{
        location:   "1.1.1.1",
        CreateTime: time.Now(),
        AccessTime: time.Now(),
        DnsRecord: &handler.Record{
            A: []handler.A_Record{
                {Ip: net.ParseIP("1.1.1.1")},
            },
        },
    }
    heap.Push(c, &item)
    time.Sleep(time.Second * 2)
    _, record := c.Get("1.1.1.1")
    if record != nil {
        t.Fail()
    }
}