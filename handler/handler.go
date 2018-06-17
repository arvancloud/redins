package handler

import (
    "encoding/json"
    "strings"
    "time"
    "math/rand"
    "sync"
    "net"

    "github.com/miekg/dns"
    "arvancloud/redins/redis"
    "github.com/patrickmn/go-cache"
    "arvancloud/redins/config"
    "arvancloud/redins/eventlog"
    "arvancloud/redins/dns_types"
    "arvancloud/redins/upstream"
    "arvancloud/redins/geoip"
    "arvancloud/redins/healthcheck"
    "github.com/coredns/coredns/request"
)

type DnsRequestHandler struct {
    DefaultTtl        int
    ZoneReload        int
    CacheTimeout      int
    LogSourceLocation bool
    UpstreamFallback  bool
    Zones             []string
    zoneLock          sync.RWMutex
    LastZoneUpdate    time.Time
    Redis             *redis.Redis
    Logger            *eventlog.EventLogger
    cache             *cache.Cache
    geoip             *geoip.GeoIp
    healthcheck       *healthcheck.Healthcheck
    upstream          *upstream.Upstream

}

type Zone struct {
    Name      string
    Locations map[string]struct{}
}

func NewHandler(config *config.RedinsConfig) *DnsRequestHandler {
    h := &DnsRequestHandler {
        DefaultTtl: config.Handler.DefaultTtl,
        ZoneReload: config.Handler.ZoneReload,
        CacheTimeout: config.Handler.CacheTimeout,
        LogSourceLocation: config.Handler.LogSourceLocation,
        UpstreamFallback: config.Handler.UpstreamFallback,
        zoneLock: sync.RWMutex{},
    }

    h.Redis = redis.NewRedis(&config.Handler.Redis)
    h.Logger = eventlog.NewLogger(&config.Handler.Log)
    h.geoip = geoip.NewGeoIp(config)
    h.healthcheck = healthcheck.NewHealthcheck(config)
    h.upstream = upstream.NewUpstream(config)

    h.LoadZones()

    h.cache = cache.New(time.Second * time.Duration(h.CacheTimeout), time.Duration(h.CacheTimeout) * time.Second * 10)

    go h.healthcheck.Start()

    return h
}

func (h *DnsRequestHandler) HandleRequest(state *request.Request) {
    qname := state.Name()
    qtype := state.QType()

    eventlog.Logger.Debugf("name : %s", state.Name())
    eventlog.Logger.Debugf("type : %s", state.Type())

    requestStartTime := time.Now()

    logData := map[string]interface{} {
        "SourceIP": state.IP(),
        "Record":   state.Name(),
        "Type":     state.Type(),
    }
    opt := state.Req.IsEdns0()
    if opt != nil && len(opt.Option) != 0 {
        logData["ClientSubnet"] = opt.Option[0].String()
    }

    if h.LogSourceLocation {
        _, _, sourceCountry, err := h.geoip.GetGeoLocation(GetSourceIp(state))
        if err == nil {
            logData["SourceCountry"] = sourceCountry
        } else {
            logData["SourceCountry"] = ""
        }
    }

    auth := true

    var record *dns_types.Record
    var res int
    var answers []dns.RR
    for {
        record, res = h.FetchRecord(qname, logData)
        if res != dns.RcodeSuccess {
            break
        }
        if len(record.CNAME.Host) == 0 {
            break
        }
        answers = append(answers, h.CNAME(qname, record)...)
        qname = record.CNAME.Host
    }

    if res == dns.RcodeSuccess {
        if qtype == dns.TypeA {
            ips := []dns_types.IP_Record{}
            if len(record.A) == 0 && record.ANAME != nil {
                upstreamAnswers, upstreamRes := h.upstream.Query(record.ANAME.Location, dns.TypeA)
                if upstreamRes == dns.RcodeSuccess {
                    answers = append(answers, upstreamAnswers...)
                } else {
                    errorResponse(state, upstreamRes)
                    h.LogRequest(logData, requestStartTime, upstreamRes)
                    return
                }
            } else {
                ips = append(ips, record.A...)
                ips = h.healthcheck.FilterHealthcheck(qname, record, ips)
                ips = h.Filter(state, record.Config.IpFilterMode, ips, logData)
                answers = append(answers, h.A(qname, ips)...)
            }
        } else if qtype == dns.TypeAAAA {
            ips := []dns_types.IP_Record{}
            if len(record.AAAA) == 0 && record.ANAME != nil {
                upstreamAnswers, upstreamRes := h.upstream.Query(record.ANAME.Location, dns.TypeAAAA)
                if upstreamRes == dns.RcodeSuccess {
                    answers = append(answers, upstreamAnswers...)
                } else {
                    errorResponse(state, upstreamRes)
                    h.LogRequest(logData, requestStartTime, upstreamRes)
                    return
                }
            } else {
                ips = append(ips, record.AAAA...)
                ips = h.healthcheck.FilterHealthcheck(qname, record, ips)
                ips = h.Filter(state, record.Config.IpFilterMode, ips, logData)
                answers = append(answers, h.AAAA(qname, ips)...)
            }
        } else {
            switch qtype {
            case dns.TypeCNAME:

            case dns.TypeTXT:
                answers = append(answers, h.TXT(qname, record)...)
            case dns.TypeNS:
                answers = append(answers, h.NS(qname, record)...)
            case dns.TypeMX:
                answers = append(answers, h.MX(qname, record)...)
            case dns.TypeSRV:
                answers = append(answers, h.SRV(qname, record)...)
            case dns.TypeSOA:
                answers = append(answers, h.SOA(qname, record)...)
            default:
                errorResponse(state, dns.RcodeNotImplemented)
                h.LogRequest(logData, requestStartTime, dns.RcodeNotImplemented)
                return
            }
        }
    } else if res == dns.RcodeNotAuth && h.UpstreamFallback {
        upstreamAnswers, upstreamRes := h.upstream.Query(qname, qtype)
        if upstreamRes == dns.RcodeSuccess {
            answers = append(answers, upstreamAnswers...)
            auth = false
        }
    } else {
        errorResponse(state, res)
        h.LogRequest(logData, requestStartTime, res)
        return
    }

    h.LogRequest(logData, requestStartTime, dns.RcodeSuccess)

    m := new(dns.Msg)
    m.SetReply(state.Req)
    m.Authoritative, m.RecursionAvailable, m.Compress = auth, h.UpstreamFallback, true

    m.Answer = append(m.Answer, answers...)

    state.SizeAndDo(m)
    m, _ = state.Scrub(m)
    state.W.WriteMsg(m)
}

func (h *DnsRequestHandler) Filter(request *request.Request, filterMode string, ips []dns_types.IP_Record, logData map[string]interface{}) []dns_types.IP_Record {
    switch  filterMode {
    case "multi":
        return ips
    case "multi_rr":
        return ShuffleIps(ips)
    case "rr":
        return GetWeightedIp(ips, logData)
    case "geo_country":
        return h.geoip.GetSameCountry(GetSourceIp(request), ips, logData)
    case "geo_location":
        return h.geoip.GetMinimumDistance(GetSourceIp(request), ips, logData)
    default:
        eventlog.Logger.Errorf("invalid filter mode : %s", filterMode)
        return ips
    }
    return ips
}

func errorResponse(state *request.Request, rcode int) {
    m := new(dns.Msg)
    m.SetRcode(state.Req, rcode)
    m.Authoritative, m.RecursionAvailable, m.Compress = true, false, true

    // m.Ns, _ = redis.SOA(state.Name(), zone, nil)

    state.SizeAndDo(m)
    state.W.WriteMsg(m)
}

func (h *DnsRequestHandler) LogRequest(data map[string]interface{}, startTime time.Time, responseCode int) {
    data["ProcessTime"] = time.Since(startTime).Nanoseconds() / 1000000
    data["ResponseCode"] = responseCode
    h.Logger.Log(data, "ar_dns_request")
}

func GetSourceIp(request *request.Request) net.IP {
    opt := request.Req.IsEdns0()
    if len(opt.Option) != 0 {
        return net.ParseIP(strings.Split(opt.Option[0].String(), "/")[0])
    }
    return net.ParseIP(request.IP())
}

func (h *DnsRequestHandler) LoadZones() {
    h.LastZoneUpdate = time.Now()
    newZones := h.Redis.GetKeys()
    h.zoneLock.Lock()
    h.Zones = newZones
    h.zoneLock.Unlock()
}

func (h *DnsRequestHandler) FetchRecord(qname string, logData map[string]interface{}) (*dns_types.Record, int) {
    cachedRecord, found := h.cache.Get(qname)
    if found {
        eventlog.Logger.Debug("cached")
        logData["Cache"] = "HIT"
        return cachedRecord.(*dns_types.Record), dns.RcodeSuccess
    } else {
        logData["Cache"] = "MISS"
        record, res := h.GetRecord(qname)
        if res == dns.RcodeSuccess {
            h.cache.Set(qname, record, time.Duration(h.CacheTimeout)*time.Second)
            return record, dns.RcodeSuccess
        }
        return nil, res
    }
}

func (h *DnsRequestHandler) A(name string, ips []dns_types.IP_Record) (answers []dns.RR) {
    for _, ip := range ips {
        if ip.Ip == nil {
            continue
        }
        r := new(dns.A)
        r.Hdr = dns.RR_Header{Name: name, Rrtype: dns.TypeA,
            Class: dns.ClassINET, Ttl: h.getTtl(ip.Ttl)}
        r.A = ip.Ip
        answers = append(answers, r)
    }
    return
}

func (h *DnsRequestHandler) AAAA(name string, ips []dns_types.IP_Record) (answers []dns.RR) {
    for _, ip := range ips {
        if ip.Ip == nil {
            continue
        }
        r := new(dns.AAAA)
        r.Hdr = dns.RR_Header{Name: name, Rrtype: dns.TypeAAAA,
            Class: dns.ClassINET, Ttl: h.getTtl(ip.Ttl)}
        r.AAAA = ip.Ip
        answers = append(answers, r)
    }
    return
}

func (h *DnsRequestHandler) CNAME(name string, record *dns_types.Record) (answers []dns.RR) {
    if len(record.CNAME.Host) == 0 {
        return
    }
    r := new(dns.CNAME)
    r.Hdr = dns.RR_Header{Name: name, Rrtype: dns.TypeCNAME,
        Class: dns.ClassINET, Ttl: h.getTtl(record.CNAME.Ttl)}
    r.Target = dns.Fqdn(record.CNAME.Host)
    answers = append(answers, r)
    return
}

func (h *DnsRequestHandler) TXT(name string, record *dns_types.Record) (answers []dns.RR) {
    for _, txt := range record.TXT {
        if len(txt.Text) == 0 {
            continue
        }
        r := new(dns.TXT)
        r.Hdr = dns.RR_Header{Name: name, Rrtype: dns.TypeTXT,
            Class: dns.ClassINET, Ttl: h.getTtl(txt.Ttl)}
        r.Txt = split255(txt.Text)
        answers = append(answers, r)
    }
    return
}

func (h *DnsRequestHandler) NS(name string, record *dns_types.Record) (answers []dns.RR) {
    for _, ns := range record.NS {
        if len(ns.Host) == 0 {
            continue
        }
        r := new(dns.NS)
        r.Hdr = dns.RR_Header{Name: name, Rrtype: dns.TypeNS,
            Class: dns.ClassINET, Ttl: h.getTtl(ns.Ttl)}
        r.Ns = ns.Host
        answers = append(answers, r)
    }
    return
}

func (h *DnsRequestHandler) MX(name string, record *dns_types.Record) (answers []dns.RR) {
    for _, mx := range record.MX {
        if len(mx.Host) == 0 {
            continue
        }
        r := new(dns.MX)
        r.Hdr = dns.RR_Header{Name: name, Rrtype: dns.TypeMX,
            Class: dns.ClassINET, Ttl: h.getTtl(mx.Ttl)}
        r.Mx = mx.Host
        r.Preference = mx.Preference
        answers = append(answers, r)
    }
    return
}

func (h *DnsRequestHandler) SRV(name string, record *dns_types.Record) (answers []dns.RR) {
    for _, srv := range record.SRV {
        if len(srv.Target) == 0 {
            continue
        }
        r := new(dns.SRV)
        r.Hdr = dns.RR_Header{Name: name, Rrtype: dns.TypeSRV,
            Class: dns.ClassINET, Ttl: h.getTtl(srv.Ttl)}
        r.Target = srv.Target
        r.Weight = srv.Weight
        r.Port = srv.Port
        r.Priority = srv.Priority
        answers = append(answers, r)
    }
    return
}

func (h *DnsRequestHandler) SOA(name string, record *dns_types.Record) (answers []dns.RR) {
    r := new(dns.SOA)
    if record.SOA.Ns == "" {
        r.Hdr = dns.RR_Header{Name: name, Rrtype: dns.TypeSOA,
            Class: dns.ClassINET, Ttl: uint32(h.DefaultTtl)}
        r.Ns = "ns1." + name
        r.Mbox = hostmaster + "." + name
        r.Refresh = 86400
        r.Retry = 7200
        r.Expire = 3600
        r.Minttl = uint32(h.DefaultTtl)
    } else {
        r.Hdr = dns.RR_Header{Name: record.ZoneName, Rrtype: dns.TypeSOA,
            Class: dns.ClassINET, Ttl: h.getTtl(record.SOA.Ttl)}
        r.Ns = record.SOA.Ns
        r.Mbox = record.SOA.MBox
        r.Refresh = record.SOA.Refresh
        r.Retry = record.SOA.Retry
        r.Expire = record.SOA.Expire
        r.Minttl = record.SOA.MinTtl
    }
    r.Serial = h.serial()
    answers = append(answers, r)
    return
}

func (h *DnsRequestHandler) serial() uint32 {
    return uint32(time.Now().Unix())
}

func (h *DnsRequestHandler) getTtl(ttl uint32) uint32 {
    defaultTtl := uint32(h.DefaultTtl)
    if ttl != 0 {
        return ttl
    } else if defaultTtl != 0 {
        return defaultTtl
    }
    return 300
}

func (h *DnsRequestHandler) findLocation(query string, z *Zone) string {
    var (
        ok                bool
        closestEncloser   string
        sourceOfSynthesis string
    )

    // request for zone records
    if query == z.Name {
        return query
    }

    query = strings.TrimSuffix(query, "."+z.Name)

    if _, ok = z.Locations[query]; ok {
        return query
    }

    closestEncloser, sourceOfSynthesis, ok = splitQuery(query)
    for ok {
        ceExists := keyMatches(closestEncloser, z) || keyExists(closestEncloser, z)
        ssExists := keyExists(sourceOfSynthesis, z)
        if ceExists {
            if ssExists {
                return sourceOfSynthesis
            } else {
                return ""
            }
        } else {
            closestEncloser, sourceOfSynthesis, ok = splitQuery(closestEncloser)
        }
    }
    return ""
}

func keyExists(key string, z *Zone) bool {
    _, ok := z.Locations[key]
    return ok
}

func keyMatches(key string, z *Zone) bool {
    for value := range z.Locations {
        if strings.HasSuffix(value, key) {
            return true
        }
    }
    return false
}

func splitQuery(query string) (string, string, bool) {
    if query == "" {
        return "", "", false
    }
    var (
        splits            []string
        closestEncloser   string
        sourceOfSynthesis string
    )
    splits = strings.SplitAfterN(query, ".", 2)
    if len(splits) == 2 {
        closestEncloser = splits[1]
        sourceOfSynthesis = "*." + closestEncloser
    } else {
        closestEncloser = ""
        sourceOfSynthesis = "*"
    }
    return closestEncloser, sourceOfSynthesis, true
}

func split255(s string) []string {
    if len(s) < 255 {
        return []string{s}
    }
    sx := []string{}
    p, i := 0, 255
    for {
        if i <= len(s) {
            sx = append(sx, s[p:i])
        } else {
            sx = append(sx, s[p:])
            break

        }
        p, i = p+255, i+255
    }

    return sx
}

func (h *DnsRequestHandler) Matches(qname string) string {
    zone := ""
    h.zoneLock.RLock()
    zones := h.Zones
    h.zoneLock.RUnlock()
    for _, zname := range zones {
        if dns.IsSubDomain(zname, qname) {
            // We want the *longest* matching zone, otherwise we may end up in a parent
            if len(zname) > len(zone) {
                zone = zname
            }
        }
    }
    return zone
}

func (h *DnsRequestHandler) GetRecord(qname string) (record *dns_types.Record, rcode int) {
    eventlog.Logger.Debug("GetRecord")

    eventlog.Logger.Debugf("%v", h.Zones)
    if time.Since(h.LastZoneUpdate) > time.Duration(h.ZoneReload) * time.Second {
        eventlog.Logger.Debug("loading zones")
        h.LoadZones()
    }
    eventlog.Logger.Debugf("%v", h.Zones)

    zone := h.Matches(qname)
    eventlog.Logger.Debugf("zone : %s", zone)
    if zone == "" {
        eventlog.Logger.Debugf("no matching zone found for %s", qname)
        return nil, dns.RcodeNotAuth
    }

    z := h.LoadZone(zone)
    if z == nil {
        eventlog.Logger.Errorf("empty zone : %s", zone)
        return nil, dns.RcodeServerFailure
    }

    location := h.findLocation(qname, z)
    if len(location) == 0 { // empty, no results
        return nil, dns.RcodeNameError
    }
    eventlog.Logger.Debugf("location : %s", location)

    record = h.GetLocation(location, z)
    if record == nil {
        return nil, dns.RcodeServerFailure
    }
    record.ZoneName = zone

    return record, dns.RcodeSuccess
}

func (h *DnsRequestHandler) LoadZone(zone string) *Zone {
    z := new(Zone)
    z.Name = zone
    vals := h.Redis.GetHKeys(zone)
    z.Locations = make(map[string]struct{})
    for _, val := range vals {
        z.Locations[val] = struct{}{}
    }

    return z
}

func (h *DnsRequestHandler) GetLocation(location string, z *Zone) *dns_types.Record {
    var label string
    if location == z.Name {
        label = "@"
    } else {
        label = location
    }
    val := h.Redis.HGet(z.Name, label)
    r := new(dns_types.Record)
    r.Config.IpFilterMode = "multi"
    r.Config.HealthCheckConfig.Enable = false
    err := json.Unmarshal([]byte(val), r)
    if err != nil {
        eventlog.Logger.Errorf("cannot parse json : %s -> %s", val, err)
        return nil
    }
    return r
}

func (h *DnsRequestHandler) SetLocation(location string, z *Zone, val *dns_types.Record) {
    jsonValue, err := json.Marshal(val)
    if err != nil {
        eventlog.Logger.Errorf("cannot encode to json : %s", err)
        return
    }
    var label string
    if location == z.Name {
        label = "@"
    } else {
        label = location
    }
    h.Redis.HSet(z.Name, label, string(jsonValue))
}

func GetWeightedIp(ips []dns_types.IP_Record, logData map[string]interface{}) []dns_types.IP_Record {
    sum := 0
    for _, ip := range ips {
        sum += ip.Weight
    }

    index := 0

    // all Ips have 0 weight, choosing a random one
    if sum == 0 {
        index = rand.Intn(len(ips))
    } else {
        x := rand.Intn(sum)
        for ; index < len(ips); index++ {
            // skip Ips with 0 weight
            x -= ips[index].Weight
            if x < 0 {
                break
            }
        }
        if index >= len(ips) {
            index--
        }
    }
    logData["DestinationIp"] = ips[index].Ip.String()
    return []dns_types.IP_Record { ips[index] }
}

func ShuffleIps(ips []dns_types.IP_Record) []dns_types.IP_Record {
    r := rand.New(rand.NewSource(time.Now().Unix()))
    ret := make([]dns_types.IP_Record, len(ips))
    perm := r.Perm(len(ips))
    for i, randIndex := range perm {
        ret[i] = ips[randIndex]
    }
    return ret
}

const (
    hostmaster = "hostmaster"
)
