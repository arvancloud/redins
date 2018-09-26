package redis

import (
    "testing"
    "fmt"
)

func TestRedis(t *testing.T) {
    cfg := RedisConfig {
        Suffix: "_redistest",
        Prefix: "redistest_",
        Port: 6379,
        Ip: "redis",
    }
    r := NewRedis(&cfg)

    r.Set("1", "1")
    if r.Get("1") != "1" {
        fmt.Println("1")
        t.Fail()
    }

    r.HSet("2", "1", "1")
    r.HSet("2", "2", "2")
    hkeys := r.GetHKeys("2")
    if hkeys[0] != "1" || hkeys[1] != "2" {
        fmt.Println("2")
        t.Fail()
    }

    if r.HGet("2", "1") != "1" {
        fmt.Println("3")
        t.Fail()
    }
    if r.HGet("2", "2") != "2" {
        fmt.Println("4")
        t.Fail()
    }

    if len(r.GetKeys("*")) != 2 {
        fmt.Println("5")
        t.Fail()
    }
    fmt.Println(r.GetKeys("*"))
    r.Del("*")
    fmt.Println(r.GetKeys("*"))
    if len(r.GetKeys("*")) != 0 {
        fmt.Println("6")
        t.Fail()
    }
}
