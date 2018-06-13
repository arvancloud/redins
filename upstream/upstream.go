package upstream

import (
    "time"
    "strconv"

    "github.com/miekg/dns"
    "github.com/patrickmn/go-cache"
    "arvancloud/redins/config"
    "arvancloud/redins/eventlog"
)

type Upstream struct {
    client        *dns.Client
    connectionStr string
    cache         *cache.Cache
}

func NewUpstream(config *config.RedinsConfig) *Upstream {
    u := &Upstream {
        client: nil,
    }

    u.client = &dns.Client {
        Net: config.Upstream.Protocol,
        Timeout: time.Duration(config.Upstream.Timeout) * time.Millisecond,
    }
    u.connectionStr = config.Upstream.Ip + ":" + strconv.Itoa(config.Upstream.Port)
    u.cache = cache.New(time.Second * time.Duration(defaultCacheTtl), time.Second * time.Duration(defaultCacheTtl) * 10)

    return u
}

func (u *Upstream) Query(location string, qtype uint16) ([]dns.RR, int) {
    key := location + ":" + strconv.Itoa(int(qtype))
    res, exp, found := u.cache.GetWithExpiration(key)
    if found {
        records := res.([]dns.RR)
        for _, record := range records {
            record.Header().Ttl = uint32(time.Until(exp).Seconds())
        }
        return records, dns.RcodeSuccess
    }
    m := new(dns.Msg)
    m.SetQuestion(location, qtype)
    r, _, err := u.client.Exchange(m, u.connectionStr)
    if err != nil {
        eventlog.Logger.Errorf("failed to retreive record from upstream %s : %s", u.connectionStr, err)
        return []dns.RR{}, dns.RcodeServerFailure
    }
    if len(r.Answer) == 0 {
        return []dns.RR{}, dns.RcodeNameError
    }
    minTtl := r.Answer[0].Header().Ttl
    for _, record := range r.Answer {
        if record.Header().Ttl < minTtl {
            minTtl = record.Header().Ttl
        }
    }
    u.cache.Set(key, r.Answer, time.Duration(minTtl) * time.Second)
    return r.Answer, dns.RcodeSuccess
}

const (
    defaultCacheTtl = 600
)