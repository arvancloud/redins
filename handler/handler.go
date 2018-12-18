package handler

import (
    "encoding/json"
    "strings"
    "time"
    "math/rand"
    "sync"
    "net"

    "github.com/miekg/dns"
    "github.com/patrickmn/go-cache"
    "github.com/coredns/coredns/request"
    "github.com/hawell/logger"
    "github.com/hawell/uperdis"
    "github.com/hashicorp/go-immutable-radix"
)

type DnsRequestHandler struct {
    Config            *HandlerConfig
    Zones             *iradix.Tree
    LastZoneUpdate    time.Time
    Redis             *uperdis.Redis
    Logger            *logger.EventLogger
    cache             *cache.Cache
    geoip             *GeoIp
    healthcheck       *Healthcheck
    upstream          *Upstream
    quit              chan struct{}
    quitWG            sync.WaitGroup

}

type HandlerConfig struct {
    Upstream []UpstreamConfig `json:"upstream,omitempty"`
    GeoIp GeoIpConfig `json:"geoip,omitempty"`
    HealthCheck HealthcheckConfig `json:"healthcheck,omitempty"`
    MaxTtl int `json:"max_ttl,omitempty"`
    CacheTimeout int `json:"cache_timeout,omitempty"`
    ZoneReload int `json:"zone_reload,omitempty"`
    LogSourceLocation bool `json:"log_source_location,omitempty"`
    UpstreamFallback bool `json:"upstream_fallback,omitempty"`
    Redis uperdis.RedisConfig `json:"redis,omitempty"`
    Log logger.LogConfig `json:"log,omitempty"`
}

func NewHandler(config *HandlerConfig) *DnsRequestHandler {
    h := &DnsRequestHandler {
        Config: config,
    }

    h.Redis = uperdis.NewRedis(&config.Redis)
    h.Logger = logger.NewLogger(&config.Log)
    h.geoip = NewGeoIp(&config.GeoIp)
    h.healthcheck = NewHealthcheck(&config.HealthCheck, h.Redis)
    h.upstream = NewUpstream(config.Upstream)
    h.Zones = iradix.New()
    h.quit = make(chan struct{}, 1)

    h.LoadZones()

    h.cache = cache.New(time.Second * time.Duration(h.Config.CacheTimeout), time.Duration(h.Config.CacheTimeout) * time.Second * 10)

    go h.healthcheck.Start()
    go h.UpdateZones()

    return h
}

func (h *DnsRequestHandler) ShutDown() {
    // fmt.Println("handler : stopping")
    h.healthcheck.ShutDown()
    h.quitWG.Add(1)
    close(h.quit)
    h.quitWG.Wait()
    // fmt.Println("handler : stopped")
}

func (h *DnsRequestHandler) UpdateZones() {
    for {
        select {
        case <-h.quit:
            // fmt.Println("updateZone : quit")
            h.quitWG.Done()
            return
        case <-time.After(time.Duration(h.Config.ZoneReload) * time.Second):
            logger.Default.Debugf("%v", h.Zones)
            logger.Default.Debug("loading zones")
            h.LoadZones()
        }
    }
}

func (h *DnsRequestHandler) HandleRequest(state *request.Request) {
    qname := state.Name()
    qtype := state.QType()

    logger.Default.Debugf("name : %s", state.Name())
    logger.Default.Debugf("type : %s", state.Type())

    requestStartTime := time.Now()

    logData := map[string]interface{} {
        "SourceIP": state.IP(),
        "Record":   state.Name(),
        "Type":     state.Type(),
    }
    logData["ClientSubnet"] = GetSourceSubnet(state)

    if h.Config.LogSourceLocation {
        _, _, sourceCountry, err := h.geoip.GetGeoLocation(GetSourceIp(state))
        if err == nil {
            logData["SourceCountry"] = sourceCountry
        } else {
            logData["SourceCountry"] = ""
        }
    }

    auth := true

    var record *Record
    var localRes int
    var res int
    var answers []dns.RR
    var authority []dns.RR
    record, localRes = h.FetchRecord(qname, logData)
    originalRecord := record
    secured := state.Do() && record != nil && record.Zone.Config.DnsSec
    if record != nil {
        logData["DomainId"] = record.Zone.Config.DomainId
        if qtype != dns.TypeCNAME {
            for {
                if localRes != dns.RcodeSuccess {
                    break
                }
                if record.CNAME == nil {
                    break
                }
                if !/*record.Zone.Config.CnameFlattening*/false {
                    answers = AppendRR(answers, h.CNAME(qname, record), qname, record, secured)
                    qname = record.CNAME.Host
                }
                record, localRes = h.FetchRecord(record.CNAME.Host, logData)
            }
        }
    }

    res = localRes
    if localRes == dns.RcodeSuccess {
        switch qtype {
        case dns.TypeA:
            if len(record.A.Data) == 0 {
                if record.ANAME != nil {
                    anameAnswer, anameRes := h.FetchRecord(record.ANAME.Location, logData)
                    if anameRes == dns.RcodeSuccess {
                        ips := h.Filter(state, &anameAnswer.A, logData)
                        answers = AppendRR(answers, h.A(qname, anameAnswer, ips), qname, record, secured)
                    } else {
                        upstreamAnswers, upstreamRes := h.upstream.Query(record.ANAME.Location, dns.TypeA)
                        if upstreamRes == dns.RcodeSuccess {
                            var anameRecord []dns.RR
                            for _, r := range upstreamAnswers {
                                if r.Header().Name == record.ANAME.Location && r.Header().Rrtype == dns.TypeA {
                                    a := r.(*dns.A)
                                    anameRecord = append(anameRecord, &dns.A{A:a.A, Hdr:dns.RR_Header{Rrtype:dns.TypeA, Name:qname,Ttl:a.Hdr.Ttl,Class:dns.ClassINET, Rdlength:0}})
                                }
                            }
                            answers = AppendRR(answers, anameRecord, qname, record, secured)
                        }
                        res = upstreamRes
                    }
                }
            } else {
                ips := h.Filter(state, &record.A, logData)
                answers = AppendRR(answers, h.A(qname, record, ips), qname, record, secured)
            }
        case dns.TypeAAAA:
            if len(record.AAAA.Data) == 0 {
                if record.ANAME != nil {
                    anameAnswer, anameRes := h.FetchRecord(record.ANAME.Location, logData)
                    if anameRes == dns.RcodeSuccess {
                        ips := h.Filter(state, &anameAnswer.AAAA, logData)
                        answers = AppendRR(answers, h.AAAA(qname, anameAnswer, ips), qname, record, secured)
                    } else {
                        upstreamAnswers, upstreamRes := h.upstream.Query(record.ANAME.Location, dns.TypeAAAA)
                        if upstreamRes == dns.RcodeSuccess {
                            var anameRecord []dns.RR
                            for _, r := range upstreamAnswers {
                                if r.Header().Name == record.ANAME.Location && r.Header().Rrtype == dns.TypeAAAA {
                                    a := r.(*dns.AAAA)
                                    anameRecord = append(anameRecord, &dns.AAAA{AAAA:a.AAAA, Hdr:dns.RR_Header{Rrtype:dns.TypeAAAA, Name:qname,Ttl:a.Hdr.Ttl,Class:dns.ClassINET, Rdlength:0}})
                                }
                            }
                            answers = AppendRR(answers, anameRecord, qname, record, secured)
                        }
                        res = upstreamRes
                    }
                }
            } else {
                ips := h.Filter(state, &record.AAAA, logData)
                answers = AppendRR(answers, h.AAAA(qname, record, ips), qname, record, secured)
            }
        case dns.TypeCNAME:
            answers = AppendRR(answers, h.CNAME(qname, record), qname, record, secured)
        case dns.TypeTXT:
            answers = AppendRR(answers, h.TXT(qname, record), qname, record, secured)
        case dns.TypeNS:
            answers = AppendRR(answers, h.NS(qname, record), qname, record, secured)
        case dns.TypeMX:
            answers = AppendRR(answers, h.MX(qname, record), qname, record, secured)
        case dns.TypeSRV:
            answers = AppendRR(answers, h.SRV(qname, record), qname, record, secured)
        case dns.TypeCAA:
            caaRecord := h.FindCAA(record)
            if caaRecord != nil {
                answers = AppendRR(answers, h.CAA(qname, caaRecord), qname, caaRecord, secured)
            }
        case dns.TypeSOA:
            answers = AppendSOA(answers, record.Zone, secured)
        case dns.TypeDNSKEY:
            if secured {
                answers = []dns.RR{record.Zone.DnsKey, record.Zone.DnsKeySig}
            }
        default:
            answers = []dns.RR{}
            authority = []dns.RR{}
            res = dns.RcodeNotImplemented
        }
        if len(answers) == 0 {
            if originalRecord.CNAME != nil {
                answers = AppendRR(answers, h.CNAME(qname, record), qname, record, secured)
            } else {
                authority = AppendSOA(authority, originalRecord.Zone, secured)
                authority = AppendNSEC(authority, originalRecord.Zone, qname, secured)
            }
        }
    } else if localRes == dns.RcodeNameError {
        answers = []dns.RR{}
        authority = AppendSOA(authority, originalRecord.Zone, secured)
        if secured {
            authority = AppendNSEC(authority, record.Zone, qname, secured)
            res = dns.RcodeSuccess
        }
    } else if localRes == dns.RcodeNotAuth {
        if  h.Config.UpstreamFallback {
            upstreamAnswers, upstreamRes := h.upstream.Query(dns.Fqdn(qname), qtype)
            if upstreamRes == dns.RcodeSuccess {
                answers = append(answers, upstreamAnswers...)
                auth = false
            }
            res = upstreamRes
        } else if originalRecord != nil && originalRecord.CNAME != nil {
            if len(answers) == 0 {
                answers = AppendRR(answers, h.CNAME(qname, record), qname, record, secured)
            }
            res = dns.RcodeSuccess
        }
    }

    h.LogRequest(logData, requestStartTime, res)
    m := new(dns.Msg)
    m.SetReply(state.Req)
    m.Authoritative, m.RecursionAvailable, m.Compress = auth, h.Config.UpstreamFallback, true
    m.SetRcode(state.Req, res)
    m.Answer = append(m.Answer, answers...)
    m.Ns = append(m.Ns, authority...)

    state.SizeAndDo(m)
    m = state.Scrub(m)
    state.W.WriteMsg(m)
}

func (h *DnsRequestHandler) Filter(request *request.Request, rrset *IP_RRSet, logData map[string]interface{}) []IP_RR {
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
        return []IP_RR{ips[index]}

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
        for _, o := range opt.Option {
            switch v := o.(type) {
            case *dns.EDNS0_SUBNET:
                return v.Address
            }
        }
    }
    return net.ParseIP(request.IP())
}

func GetSourceSubnet(request *request.Request) string {
    opt := request.Req.IsEdns0()
    if opt != nil && len(opt.Option) != 0 {
        for _, o := range opt.Option {
            switch o.(type) {
            case *dns.EDNS0_SUBNET:
                return o.String()
            }
        }
    }
    return ""
}

func reverseZone(zone string) string {
    x := strings.Split(zone, ".")
    var y string
    for i := len(x)-1; i > 0; i-- {
        y += x[i] + "."
    }
    y += x[0]
    return y
}

func (h *DnsRequestHandler) LoadZones() {
    h.LastZoneUpdate = time.Now()
    zones := h.Redis.GetKeys("*")
    newZones := iradix.New()
    for _, zone := range zones {
        newZones, _, _ = newZones.Insert([]byte(reverseZone(zone)), zone)
    }
    h.Zones = newZones
}

func (h *DnsRequestHandler) FetchRecord(qname string, logData map[string]interface{}) (*Record, int) {
    cachedRecord, found := h.cache.Get(qname)
    if found {
        logger.Default.Debug("cached")
        logData["Cache"] = "HIT"
        return cachedRecord.(*Record), dns.RcodeSuccess
    } else {
        logData["Cache"] = "MISS"
        record, res := h.GetRecord(qname)
        if res == dns.RcodeSuccess {
            h.cache.Set(qname, record, time.Duration(h.Config.CacheTimeout)*time.Second)
        }
        return record, res
    }
}

func (h *DnsRequestHandler) A(name string, record *Record, ips []IP_RR) (answers []dns.RR) {
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

func (h *DnsRequestHandler) AAAA(name string, record *Record, ips []IP_RR) (answers []dns.RR) {
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

func (h *DnsRequestHandler) CNAME(name string, record *Record) (answers []dns.RR) {
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

func (h *DnsRequestHandler) TXT(name string, record *Record) (answers []dns.RR) {
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

func (h *DnsRequestHandler) NS(name string, record *Record) (answers []dns.RR) {
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

func (h *DnsRequestHandler) MX(name string, record *Record) (answers []dns.RR) {
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

func (h *DnsRequestHandler) SRV(name string, record *Record) (answers []dns.RR) {
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

func (h *DnsRequestHandler) CAA(name string, record *Record) (answers []dns.RR) {
    for _, caa := range record.CAA.Data {
        r := new(dns.CAA)
        r.Hdr = dns.RR_Header{Name: name, Rrtype: dns.TypeCAA,
            Class: dns.ClassINET, Ttl: h.getTtl(record.CAA.Ttl)}
        r.Value = caa.Value
        r.Flag = caa.Flag
        r.Tag = caa.Tag
        answers = append(answers, r)
    }
    return
}

func (h *DnsRequestHandler) getTtl(ttl uint32) uint32 {
    maxTtl := uint32(h.Config.MaxTtl)
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
    rzname := reverseZone(qname)
    _, zname, ok := h.Zones.Root().LongestPrefix([]byte(rzname))
    if ok {
        return zname.(string)
    }
    return ""
}

func (h *DnsRequestHandler) GetRecord(qname string) (record *Record, rcode int) {
    logger.Default.Debug("GetRecord")

    zone := h.Matches(qname)
    logger.Default.Debugf("zone : %s", zone)
    if zone == "" {
        logger.Default.Debugf("no matching zone found for %s", qname)
        return nil, dns.RcodeNotAuth
    }

    z := h.LoadZone(zone)
    if z == nil {
        logger.Default.Errorf("empty zone : %s", zone)
        return nil, dns.RcodeServerFailure
    }

    location := h.findLocation(qname, z)
    if len(location) == 0 { // empty, no results
        return &Record{Name:qname, Zone: z}, dns.RcodeNameError
    }
    logger.Default.Debugf("location : %s", location)

    record = h.LoadLocation(location, z)
    if record == nil {
        return nil, dns.RcodeServerFailure
    }

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

    z.Config = ZoneConfig {
        DnsSec: false,
        CnameFlattening: false,
        SOA: &SOA_RRSet {
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
            logger.Default.Errorf("cannot parse zone config : %s", err)
        }
    }
    z.Config.SOA.Ns = dns.Fqdn(z.Config.SOA.Ns)
    if z.Config.DnsSec {
        pubStr := h.Redis.Get(z.Name + "_pub")
        privStr := h.Redis.Get(z.Name + "_priv")
        privStr = strings.Replace(privStr, "\\n", "\n", -1)
        if pubStr == "" || privStr == "" {
            logger.Default.Errorf("key is not set for zone %s", z.Name)
            z.Config.DnsSec = false
            return z
        }
        if rr, err := dns.NewRR(pubStr); err == nil {
            z.DnsKey = rr.(*dns.DNSKEY)
        } else {
            logger.Default.Errorf("cannot parse zone key : %s", err)
            z.Config.DnsSec = false
            return z
        }
        if pk, err := z.DnsKey.NewPrivateKey(privStr); err == nil {
            z.PrivateKey = pk
        } else {
            logger.Default.Errorf("cannot create private key : %s", err)
            z.Config.DnsSec = false
            return z
        }
        now := time.Now()
        z.KeyInception = uint32(now.Add(-3 * time.Hour).Unix())
        z.KeyExpiration = uint32(now.Add(8 * 24 * time.Hour).Unix())
        if rrsig, err := Sign([]dns.RR{z.DnsKey}, z.Name, z, 300); err == nil {
            z.DnsKeySig = rrsig
        } else {
            logger.Default.Errorf("cannot create RRSIG for DNSKEY : %s", err)
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
    return z
}


func (h *DnsRequestHandler) LoadLocation(location string, z *Zone) *Record {
    var label, name string
    if location == z.Name {
        name = z.Name
        label = "@"
    } else {
        name = location + "." + z.Name
        label = location
    }
    r := new(Record)
    r.A = IP_RRSet{
        FilterConfig: IpFilterConfig {
            Count: "multi",
            Order: "none",
            GeoFilter: "none",
        },
        HealthCheckConfig: IpHealthCheckConfig {
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
        logger.Default.Errorf("cannot parse json : zone -> %s, %s -> %s", z.Name, val, err)
        return nil
    }

    return r
}

func (h *DnsRequestHandler) SetLocation(location string, z *Zone, val *Record) {
    jsonValue, err := json.Marshal(val)
    if err != nil {
        logger.Default.Errorf("cannot encode to json : %s", err)
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

func ChooseIp(ips []IP_RR, weighted bool) int {
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

func AppendRR(answers []dns.RR, rrs []dns.RR, qname string, record *Record, secured bool) []dns.RR {
    if len(rrs) == 0 {
        return answers
    }
    answers = append(answers, rrs...)
    if secured {
        if rrsig, err := Sign(rrs, qname, record.Zone, rrs[0].Header().Ttl); err == nil {
            answers = append(answers, rrsig)
        }
    }
    return answers
}

func AppendSOA(target []dns.RR, zone *Zone, secured bool) []dns.RR {
    target = append(target, zone.Config.SOA.Data)
    if secured {
        if rrsig, err := Sign([]dns.RR{zone.Config.SOA.Data}, zone.Name, zone, zone.Config.SOA.Ttl); err == nil {
            target = append(target, rrsig)
        }
    }
    return target
}

func AppendNSEC(target []dns.RR, zone *Zone, qname string, secured bool) []dns.RR {
    if !secured {
        return target
    }
    if nsec, err := NSec(qname, zone); err == nil {
        target = append(target, nsec...)
    }
    return target
}

func (h *DnsRequestHandler) FindCAA(record *Record) *Record {
    zone := record.Zone
    currentRecord := record
    for currentRecord != nil && strings.HasSuffix(currentRecord.Name, zone.Name) {
        for {
            if currentRecord == nil {
                return nil
            }
            if currentRecord.CNAME == nil {
                break
            }
            currentRecord, _ = h.FetchRecord(currentRecord.CNAME.Host, map[string]interface{}{})
        }
        if len(currentRecord.CAA.Data) != 0 {
            return currentRecord
        }
        splits := strings.SplitAfterN(currentRecord.Name, ".", 2)
        if len(splits) != 2 {
            return nil
        }
        currentRecord, _ = h.FetchRecord(splits[1], map[string]interface{}{})
    }
    return nil
}
