package handler

import (
    "testing"
    "log"

    "github.com/miekg/dns"
    "github.com/coredns/coredns/plugin/test"
    "github.com/coredns/coredns/plugin/pkg/dnstest"
    "github.com/coredns/coredns/request"
    "github.com/hawell/uperdis"
    "github.com/hawell/logger"
)

var upstreamTestConfig = HandlerConfig {
    MaxTtl: 300,
    CacheTimeout: 60,
    ZoneReload: 600,
    UpstreamFallback: true,
    Redis: uperdis.RedisConfig {
        Ip: "redis",
        Port: 6379,
        DB: 0,
        Password: "",
        Prefix: "test_",
        Suffix: "_test",
        ConnectTimeout: 0,
        ReadTimeout: 0,
    },
    Log: logger.LogConfig {
        Enable: false,
    },
    Upstream: []UpstreamConfig  {
        {
            Ip: "1.1.1.1",
            Port: 53,
            Protocol: "udp",
            Timeout: 1000,
        },
    },
    GeoIp: GeoIpConfig {
        Enable: true,
        CountryDB: "../geoCity.mmdb",
    },
}

func TestUpstream(t *testing.T) {
    logger.Default = logger.NewLogger(&logger.LogConfig{})
    u := NewUpstream(upstreamTestConfig.Upstream)
    rs, res := u.Query("google.com.", dns.TypeAAAA)
    if len(rs) == 0 || res != 0 {
        log.Printf("[ERROR] AAAA failed")
        t.Fail()
    }
    rs, res = u.Query("google.com.", dns.TypeA)
    if len(rs) == 0 || res != 0 {
        log.Printf("[ERROR] A failed")
        t.Fail()
    }
    rs, res = u.Query("google.com.", dns.TypeTXT)
    if len(rs) == 0 || res != 0 {
        log.Printf("[ERROR] TXT failed")
        t.Fail()
    }
}

func TestFallback(t *testing.T) {
    tc := test.Case{
        Qname: "google.com.", Qtype: dns.TypeAAAA,
    }
    logger.Default = logger.NewLogger(&logger.LogConfig{})

    h := NewHandler(&upstreamTestConfig)

    r := tc.Msg()
    w := dnstest.NewRecorder(&test.ResponseWriter{})
    state := request.Request{W: w, Req: r}
    h.HandleRequest(&state)

    resp := w.Msg

    if resp.Rcode != dns.RcodeSuccess {
        t.Fail()
    }
}
