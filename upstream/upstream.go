package upstream

import (
    "time"
    "log"
    "strconv"

    "github.com/miekg/dns"
    "github.com/go-ini/ini"
    "github.com/patrickmn/go-cache"
)

type Upstream struct {
    config        *UpstreamConfig
    Enabled       bool
    client        *dns.Client
    connectionStr string
    cache         *cache.Cache
}

type UpstreamConfig struct {
    Enable   bool
    ip       string
    port     int
    protocol string
}

func LoadConfig(cfg *ini.File, section string) *UpstreamConfig {
    upstreamConfig := cfg.Section(section)
    return &UpstreamConfig {
        Enable: upstreamConfig.Key("enable").MustBool(false),
        ip: upstreamConfig.Key("ip").MustString("1.1.1.1"),
        port: upstreamConfig.Key("port").MustInt(53),
        protocol: upstreamConfig.Key("protocol").In("udp", []string {"tcp", "udp"}),
    }
}

func NewUpstream(config *UpstreamConfig) *Upstream {
    u := &Upstream {
        config: config,
        client: nil,
        Enabled: config.Enable,
    }

    if u.config.Enable == false {
        return u
    }

    u.client = &dns.Client {
        Net: config.protocol,
    }
    u.connectionStr = config.ip + ":" + strconv.Itoa(config.port)
    u.cache = cache.New(time.Second * time.Duration(defaultCacheTtl), time.Second * time.Duration(defaultCacheTtl) * 10)

    return u
}

func (u *Upstream) Query(location string, qtype uint16) []dns.RR {
    if u.Enabled == false {
        return []dns.RR{}
    }
    key := location + ":" + strconv.Itoa(int(qtype))
    res, exp, found := u.cache.GetWithExpiration(key)
    if found {
        records := res.([]dns.RR)
        for _, record := range records {
            record.Header().Ttl = uint32(time.Until(exp).Seconds())
        }
        return records
    }
    m := new(dns.Msg)
    m.SetQuestion(location, qtype)
    r, _, err := u.client.Exchange(m, u.connectionStr)
    if err != nil {
        log.Printf("[ERROR] failed to retreive record from upstream %s : %s", u.connectionStr, err)
        return []dns.RR{}
    }
    if len(r.Answer) == 0 {
        return []dns.RR{}
    }
    minTtl := r.Answer[0].Header().Ttl
    for _, record := range r.Answer {
        if record.Header().Ttl < minTtl {
            minTtl = record.Header().Ttl
        }
    }
    u.cache.Set(key, r.Answer, time.Duration(minTtl) * time.Second)
    return r.Answer
}

const (
    defaultCacheTtl = 600
)