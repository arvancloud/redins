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
    MaxTtl            int
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

func NewHandler(config *config.RedinsConfig) *DnsRequestHandler {
    h := &DnsRequestHandler {
        MaxTtl: config.Handler.MaxTtl,
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
    var localRes int
    var res int
    var answers []dns.RR
    var authority []dns.RR
    record, localRes = h.FetchRecord(qname, logData)
    originalRecord := record
    secured := state.Do() && record != nil && record.Zone.Config.DnsSec
    if record != nil && record.Zone.Config.CnameFlattening && qtype != dns.TypeCNAME {
        for {
            if localRes != dns.RcodeSuccess {
                break
            }
            if record.CNAME == nil {
                break
            }
            answers, authority = AppendRR(answers, authority, h.CNAME(qname, record), qname, record, secured, record.CNAME.RRSig)
            qname = record.CNAME.Host
            record, localRes = h.FetchRecord(qname, logData)
        }
    }

    res = localRes
    if localRes == dns.RcodeSuccess {
        switch qtype {
        case dns.TypeA:
            if len(record.A.Data) == 0 {
                if record.ANAME != nil {
                    upstreamAnswers, upstreamRes := h.upstream.Query(record.ANAME.Location, dns.TypeA)
                    if upstreamRes == dns.RcodeSuccess {
                        answers = append(answers, upstreamAnswers...)
                    }
                    res = upstreamRes
                } else {
                    answers = []dns.RR{}
                    authority = AppendSOA(authority, originalRecord.Zone, secured)
                    authority = AppendNSEC(authority, record.Zone, qname, secured)
                }
            } else {
                ips := h.Filter(state, &record.A, logData)
                answers, authority = AppendRR(answers, authority, h.A(qname, record, ips), qname, record, secured, nil)
            }
        case dns.TypeAAAA:
            if len(record.AAAA.Data) == 0 {
                if record.ANAME != nil {
                    upstreamAnswers, upstreamRes := h.upstream.Query(record.ANAME.Location, dns.TypeAAAA)
                    if upstreamRes == dns.RcodeSuccess {
                        answers = append(answers, upstreamAnswers...)
                    }
                    res = upstreamRes
                } else {
                    answers = []dns.RR{}
                    authority = AppendSOA(authority, originalRecord.Zone, secured)
                    authority = AppendNSEC(authority, record.Zone, qname, secured)
                }
            } else {
                ips := h.Filter(state, &record.AAAA, logData)
                answers, authority = AppendRR(answers, authority, h.AAAA(qname, record, ips), qname, record, secured, nil)
            }
        case dns.TypeCNAME:
            if record.CNAME == nil {
                answers = []dns.RR{}
                authority = AppendSOA(authority, originalRecord.Zone, secured)
                authority = AppendNSEC(authority, record.Zone, qname, secured)
            } else {
                answers, authority = AppendRR(answers, authority, h.CNAME(qname, record), qname, record, secured, record.CNAME.RRSig)
            }
        case dns.TypeTXT:
            if len(record.TXT.Data) == 0 {
                answers = []dns.RR{}
                authority = AppendSOA(authority, originalRecord.Zone, secured)
                authority = AppendNSEC(authority, record.Zone, qname, secured)
            } else {
                answers, authority = AppendRR(answers, authority, h.TXT(qname, record), qname, record, secured, record.TXT.RRSig)
            }
        case dns.TypeNS:
            if len(record.NS.Data) == 0 {
                answers = []dns.RR{}
                authority = AppendSOA(authority, originalRecord.Zone, secured)
                authority = AppendNSEC(authority, record.Zone, qname, secured)
            } else {
                answers, authority = AppendRR(answers, authority, h.NS(qname, record), qname, record, secured, record.NS.RRSig)
            }
        case dns.TypeMX:
            if len(record.MX.Data) == 0 {
                answers = []dns.RR{}
                authority = AppendSOA(authority, originalRecord.Zone, secured)
                authority = AppendNSEC(authority, record.Zone, qname, secured)
            } else {
                answers, authority = AppendRR(answers, authority, h.MX(qname, record), qname, record, secured, record.MX.RRSig)
            }
        case dns.TypeSRV:
            if len(record.SRV.Data) == 0 {
                answers = []dns.RR{}
                authority = AppendSOA(authority, originalRecord.Zone, secured)
                authority = AppendNSEC(authority, record.Zone, qname, secured)
            } else {
                answers, authority = AppendRR(answers, authority, h.SRV(qname, record), qname, record, secured, record.SRV.RRSig)
            }
        case dns.TypeSOA:
            answers = []dns.RR{record.Zone.Config.SOA.Data}
            if secured {
                answers = append(answers, record.Zone.Config.SOA.RRSig)
            }
        case dns.TypeDNSKEY:
            if secured {
                answers = []dns.RR{record.Zone.DnsKey, record.Zone.DnsKeySig}
            }
        default:
            answers = []dns.RR{}
            authority = []dns.RR{}
            res = dns.RcodeNotImplemented
        }
    } else if localRes == dns.RcodeNameError {
        answers = []dns.RR{}
        authority = AppendSOA(authority, originalRecord.Zone, secured)
        if secured {
            authority = AppendNSEC(authority, record.Zone, qname, secured)
            res = dns.RcodeSuccess
        }
    } else if localRes == dns.RcodeNotAuth && h.UpstreamFallback {
        upstreamAnswers, upstreamRes := h.upstream.Query(qname, qtype)
        if upstreamRes == dns.RcodeSuccess {
            answers = append(answers, upstreamAnswers...)
            auth = false
        }
        res = upstreamRes
    }

    h.LogRequest(logData, requestStartTime, res)
    m := new(dns.Msg)
    m.SetReply(state.Req)
    m.Authoritative, m.RecursionAvailable, m.Compress = auth, h.UpstreamFallback, true
    m.SetRcode(state.Req, res)
    m.Answer = append(m.Answer, answers...)
    m.Ns = append(m.Ns, authority...)

    state.SizeAndDo(m)
    m, _ = state.Scrub(m)
    state.W.WriteMsg(m)
}

func (h *DnsRequestHandler) Filter(request *request.Request, rrset *dns_types.IP_RRSet, logData map[string]interface{}) []dns_types.IP_RR {
    ips := h.healthcheck.FilterHealthcheck(request.Name(), rrset)
    switch rrset.FilterConfig.GeoFilter {
    case "country":
        ips = h.geoip.GetSameCountry(GetSourceIp(request), ips, logData)
    case "location":
        ips = h.geoip.GetMinimumDistance(GetSourceIp(request), ips, logData)
    default:
    }
    if len(ips) <= 1 {
        return ips
    }

    switch rrset.FilterConfig.Count {
    case "single":
        index := 0
        switch rrset.FilterConfig.Order {
        case "weighted":
            index = ChooseIp(ips, true)
        case "rr":
            index = ChooseIp(ips, false)
        default:
            index = 0
        }
        logData["DestinationIp"] = ips[index].Ip.String()
        logData["DestinationCountry"] = ips[index].Country
        return []dns_types.IP_RR{ips[index]}

    case "multi":
        fallthrough
    default:
        index := 0
        switch rrset.FilterConfig.Order {
        case "weighted":
            index = ChooseIp(ips, true)
        case "rr":
            index = ChooseIp(ips,false)
        default:
            index = 0
        }
        return append(ips[index:], ips[:index]...)
    }
    return ips
}

func (h *DnsRequestHandler) LogRequest(data map[string]interface{}, startTime time.Time, responseCode int) {
    data["ProcessTime"] = time.Since(startTime).Nanoseconds() / 1000000
    data["ResponseCode"] = responseCode
    h.Logger.Log(data, "ar_dns_request")
}

func GetSourceIp(request *request.Request) net.IP {
    opt := request.Req.IsEdns0()
    if opt != nil && len(opt.Option) != 0 {
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
        if record != nil {
            h.cache.Set(qname, record, time.Duration(h.CacheTimeout)*time.Second)
        }
        return record, res
    }
}

func (h *DnsRequestHandler) A(name string, record *dns_types.Record, ips []dns_types.IP_RR) (answers []dns.RR) {
    for _, ip := range ips {
        if ip.Ip == nil {
            continue
        }
        r := new(dns.A)
        r.Hdr = dns.RR_Header{Name: name, Rrtype: dns.TypeA,
            Class: dns.ClassINET, Ttl: h.getTtl(record.A.Ttl)}
        r.A = ip.Ip
        answers = append(answers, r)
    }
    return
}

func (h *DnsRequestHandler) AAAA(name string, record *dns_types.Record, ips []dns_types.IP_RR) (answers []dns.RR) {
    for _, ip := range ips {
        if ip.Ip == nil {
            continue
        }
        r := new(dns.AAAA)
        r.Hdr = dns.RR_Header{Name: name, Rrtype: dns.TypeAAAA,
            Class: dns.ClassINET, Ttl: h.getTtl(record.AAAA.Ttl)}
        r.AAAA = ip.Ip
        answers = append(answers, r)
    }
    return
}

func (h *DnsRequestHandler) CNAME(name string, record *dns_types.Record) (answers []dns.RR) {
    if record.CNAME == nil {
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
    for _, txt := range record.TXT.Data {
        if len(txt.Text) == 0 {
            continue
        }
        r := new(dns.TXT)
        r.Hdr = dns.RR_Header{Name: name, Rrtype: dns.TypeTXT,
            Class: dns.ClassINET, Ttl: h.getTtl(record.TXT.Ttl)}
        r.Txt = split255(txt.Text)
        answers = append(answers, r)
    }
    return
}

func (h *DnsRequestHandler) NS(name string, record *dns_types.Record) (answers []dns.RR) {
    for _, ns := range record.NS.Data {
        if len(ns.Host) == 0 {
            continue
        }
        r := new(dns.NS)
        r.Hdr = dns.RR_Header{Name: name, Rrtype: dns.TypeNS,
            Class: dns.ClassINET, Ttl: h.getTtl(record.NS.Ttl)}
        r.Ns = ns.Host
        answers = append(answers, r)
    }
    return
}

func (h *DnsRequestHandler) MX(name string, record *dns_types.Record) (answers []dns.RR) {
    for _, mx := range record.MX.Data {
        if len(mx.Host) == 0 {
            continue
        }
        r := new(dns.MX)
        r.Hdr = dns.RR_Header{Name: name, Rrtype: dns.TypeMX,
            Class: dns.ClassINET, Ttl: h.getTtl(record.MX.Ttl)}
        r.Mx = mx.Host
        r.Preference = mx.Preference
        answers = append(answers, r)
    }
    return
}

func (h *DnsRequestHandler) SRV(name string, record *dns_types.Record) (answers []dns.RR) {
    for _, srv := range record.SRV.Data {
        if len(srv.Target) == 0 {
            continue
        }
        r := new(dns.SRV)
        r.Hdr = dns.RR_Header{Name: name, Rrtype: dns.TypeSRV,
            Class: dns.ClassINET, Ttl: h.getTtl(record.SRV.Ttl)}
        r.Target = srv.Target
        r.Weight = srv.Weight
        r.Port = srv.Port
        r.Priority = srv.Priority
        answers = append(answers, r)
    }
    return
}

func (h *DnsRequestHandler) getTtl(ttl uint32) uint32 {
    maxTtl := uint32(h.MaxTtl)
    if ttl == 0 {
        return maxTtl
    }
    if maxTtl == 0 {
        return ttl
    }
    if ttl > maxTtl {
        return maxTtl
    }
    return ttl
}

func (h *DnsRequestHandler) findLocation(query string, z *dns_types.Zone) string {
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

func keyExists(key string, z *dns_types.Zone) bool {
    _, ok := z.Locations[key]
    return ok
}

func keyMatches(key string, z *dns_types.Zone) bool {
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
    var sx []string
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
        return &dns_types.Record{Name:qname, Zone: z}, dns.RcodeNameError
    }
    eventlog.Logger.Debugf("location : %s", location)

    record = h.LoadLocation(location, z)
    if record == nil {
        return nil, dns.RcodeServerFailure
    }

    return record, dns.RcodeSuccess
}

func (h *DnsRequestHandler) LoadZone(zone string) *dns_types.Zone {
    z := new(dns_types.Zone)
    z.Name = zone
    vals := h.Redis.GetHKeys(zone)
    z.Locations = make(map[string]struct{})
    for _, val := range vals {
        z.Locations[val] = struct{}{}
    }

    z.Config = dns_types.ZoneConfig {
        DnsSec: false,
        CnameFlattening: false,
        SOA: &dns_types.SOA_RRSet {
            Ns: "ns1." + z.Name,
            MinTtl: 300,
            Refresh: 86400,
            Retry: 7200,
            Expire: 3600,
            MBox: "hostmaster." + z.Name,
        },
    }
    z.Config.SOA.Ttl = 300
    val := h.Redis.HGet(zone, "@config")
    if len(val) > 0 {
        err := json.Unmarshal([]byte(val), &z.Config)
        if err != nil {
            eventlog.Logger.Errorf("cannot parse zone config : %s", err)
        }
    }
    if z.Config.DnsSec {
        pubStr := h.Redis.Get(z.Name + "_pub")
        privStr := h.Redis.Get(z.Name + "_priv")
        privStr = strings.Replace(privStr, "\\n", "\n", -1)
        if pubStr == "" || privStr == "" {
            eventlog.Logger.Errorf("key is not set for zone %s", z.Name)
            z.Config.DnsSec = false
            return z
        }
        if rr, err := dns.NewRR(pubStr); err == nil {
            z.DnsKey = rr.(*dns.DNSKEY)
        } else {
            eventlog.Logger.Errorf("cannot parse zone key : %s", err)
            z.Config.DnsSec = false
            return z
        }
        if pk, err := z.DnsKey.NewPrivateKey(privStr); err == nil {
            z.PrivateKey = pk
        } else {
            eventlog.Logger.Errorf("cannot create private key : %s", err)
            z.Config.DnsSec = false
            return z
        }
        now := time.Now()
        z.KeyInception = uint32(now.Add(-3 * time.Hour).Unix())
        z.KeyExpiration = uint32(now.Add(8 * 24 * time.Hour).Unix())
        if rrsig, err := Sign([]dns.RR{z.DnsKey}, z.Name, z, 300); err == nil {
            z.DnsKeySig = rrsig
        } else {
            eventlog.Logger.Errorf("cannot create RRSIG for DNSKEY : %s", err)
        }
    }
    z.Config.SOA.Data = &dns.SOA {
        Hdr: dns.RR_Header { Name: z.Name, Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: z.Config.SOA.Ttl, Rdlength:0},
        Ns: z.Config.SOA.Ns,
        Mbox: z.Config.SOA.MBox,
        Refresh: z.Config.SOA.Refresh,
        Retry: z.Config.SOA.Retry,
        Expire: z.Config.SOA.Expire,
        Minttl: z.Config.SOA.MinTtl,
        Serial: uint32(time.Now().Unix()),
    }
    if z.Config.DnsSec {
        if rrsig, err := Sign([]dns.RR{z.Config.SOA.Data}, z.Name, z, z.Config.SOA.Ttl); err == nil {
            z.Config.SOA.RRSig = rrsig
        }
    }
    return z
}


func (h *DnsRequestHandler) LoadLocation(location string, z *dns_types.Zone) *dns_types.Record {
    var label, name string
    if location == z.Name {
        name = z.Name
        label = "@"
    } else {
        name = location + "." + z.Name
        label = location
    }
    r := new(dns_types.Record)
    r.A = dns_types.IP_RRSet{
        FilterConfig: dns_types.IpFilterConfig {
            Count: "multi",
            Order: "none",
            GeoFilter: "none",
        },
        HealthCheckConfig: dns_types.IpHealthCheckConfig {
            Enable: false,
        },
    }
    r.AAAA = r.A
    r.Zone = z
    r.Name = name

    val := h.Redis.HGet(z.Name, label)
    if val == "" && name == z.Name {
        return r
    }
    err := json.Unmarshal([]byte(val), r)
    if err != nil {
        eventlog.Logger.Errorf("cannot parse json : %s -> %s", val, err)
        return nil
    }

    if z.Config.DnsSec {
        h.SignLocation(r)
    }

    return r
}

func (h *DnsRequestHandler) SetLocation(location string, z *dns_types.Zone, val *dns_types.Record) {
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

func ChooseIp(ips []dns_types.IP_RR, weighted bool) int {
    sum := 0

    if weighted == false {
        return rand.Intn(len(ips))
    }

    for _, ip := range ips {
        sum += ip.Weight
    }
    index := 0

    // all Ips have 0 weight, choosing a random one
    if sum == 0 {
        return rand.Intn(len(ips))
    }

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

    return index
}

func AppendRR(answers, authority []dns.RR, rrs []dns.RR, qname string, record *dns_types.Record, secured bool, rrsig dns.RR) ([]dns.RR, []dns.RR) {
    if len(rrs) == 0 {
        return answers, authority
    }
    answers = append(answers, rrs...)
    if secured {
        if rrsig != nil && qname == record.Name {
            qsig := dns.Copy(rrsig)
            qsig.Header().Name = qname
            answers = append(answers, qsig)
        } else {
            if rrsig, err := Sign(rrs, qname, record.Zone, rrs[0].Header().Ttl); err == nil {
                answers = append(answers, rrsig)
            }
        }
        /*
        if qname != record.Name {
            authority = AppendNSEC(authority, record.Zone, qname, NSecTypesNone, secured)
        }
        */
    }
    return answers, authority
}

func AppendSOA(target []dns.RR, zone *dns_types.Zone, secured bool) []dns.RR {
    target = append(target, zone.Config.SOA.Data)
    if secured {
        target = append(target, zone.Config.SOA.RRSig)
    }
    return target
}

func AppendNSEC(target []dns.RR, zone *dns_types.Zone, qname string, secured bool) []dns.RR {
    if !secured {
        return target
    }
    if nsec, err := NSec(qname, zone); err == nil {
        target = append(target, nsec...)
    }
    return target
}
