package upstream

import (
    "time"
    "strconv"

    "github.com/miekg/dns"
    "github.com/patrickmn/go-cache"
    "arvancloud/redins/config"
    "arvancloud/redins/eventlog"
)

type UpstreamConnection struct {
    client        *dns.Client
    connectionStr string
}

type Upstream struct {
    connections   []*UpstreamConnection
    cache         *cache.Cache
}

func NewUpstream(config *config.RedinsConfig) *Upstream {
    u := &Upstream{}

    u.cache = cache.New(time.Second * time.Duration(defaultCacheTtl), time.Second * time.Duration(defaultCacheTtl) * 10)
    for _, upstreamConfig := range config.Upstream {
        client := &dns.Client {
            Net: upstreamConfig.Protocol,
            Timeout: time.Duration(upstreamConfig.Timeout) * time.Millisecond,
        }
        connectionStr := upstreamConfig.Ip + ":" + strconv.Itoa(upstreamConfig.Port)
        connection := &UpstreamConnection {
            client: client,
            connectionStr: connectionStr,
        }
        u.connections = append(u.connections, connection)
    }

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
    for _, c := range u.connections {
        r, _, err := c.client.Exchange(m, c.connectionStr)
        if err != nil {
            eventlog.Logger.Error("failed to retreive record from upstream ", c.connectionStr, " : ", err)
            continue
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
        u.connections[0], c = c, u.connections[0]

        return r.Answer, dns.RcodeSuccess
    }
    return []dns.RR{}, dns.RcodeServerFailure
}

const (
    defaultCacheTtl = 600
)