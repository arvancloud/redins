package main

import (
    "log"
    "sync"
    "strings"
    "os"
    "net"

    "github.com/miekg/dns"
    "github.com/go-ini/ini"
    "github.com/coredns/coredns/request"
    "arvancloud/redins/handler"
    "arvancloud/redins/eventlog"
    "arvancloud/redins/geoip"
    "arvancloud/redins/server"
    "arvancloud/redins/healthcheck"
)

var (
    s *dns.Server
    h *handler.DnsRequestHandler
    l *eventlog.EventLogger
    g *geoip.GeoIp
    k *healthcheck.Healthcheck

)

func GetSourceIp(request *request.Request) net.IP {
    opt := request.Req.IsEdns0()
    if len(opt.Option) != 0 {
        return net.ParseIP(strings.Split(opt.Option[0].String(), "/")[0])
    }
    return net.ParseIP(request.IP())
}

func LogRequest(request *request.Request) {
    type RequestLogData struct {
        SourceIP     string
        Record       string
        ClientSubnet string
    }

    data := RequestLogData{
        SourceIP: request.IP(),
        Record:   request.Name(),
    }

    opt := request.Req.IsEdns0()
    if opt != nil && len(opt.Option) != 0 {
        data.ClientSubnet = opt.Option[0].String()
    }
    l.Log(data, "request")
}

func handleRequest(w dns.ResponseWriter, r *dns.Msg) {
    // log.Printf("[DEBUG] handle request")
    state := request.Request{W: w, Req: r}

    qname := state.Name()
    qtype := state.QType()

    // log.Printf("[DEBUG] name : %s", state.Name())
    // log.Printf("[DEBUG] type : %s", state.Type())

    LogRequest(&state)

    record, res := h.FetchRecord(qname)

    if res != dns.RcodeSuccess {
        errorResponse(state, res)
        return
    }

    var answers []dns.RR

    if qtype == dns.TypeA {
        ips := []handler.IP_Record{}
        ips = append(ips, record.A...)
        ips = k.FilterHealthcheck(qname, record, ips)
        ips = Filter(&state, record.ZoneCfg.IpFilterMode, ips)
        answers = h.A(qname, ips)
    } else if qtype == dns.TypeAAAA {
        ips := []handler.IP_Record{}
        ips = append(ips, record.A...)
        ips = k.FilterHealthcheck(qname, record, ips)
        ips = Filter(&state, record.ZoneCfg.IpFilterMode, ips)
        answers = h.AAAA(qname, ips)
    } else {
        switch qtype {
        case dns.TypeCNAME:
            answers = h.CNAME(qname, record)
        case dns.TypeTXT:
            answers = h.TXT(qname, record)
        case dns.TypeNS:
            answers = h.NS(qname, record)
        case dns.TypeMX:
            answers = h.MX(qname, record)
        case dns.TypeSRV:
            answers = h.SRV(qname, record)
        case dns.TypeSOA:
            answers = h.SOA(qname, record)
        default:
            errorResponse(state, dns.RcodeNotImplemented)
            return
        }
    }
    m := new(dns.Msg)
    m.SetReply(r)
    m.Authoritative, m.RecursionAvailable, m.Compress = true, false, true

    m.Answer = append(m.Answer, answers...)

    state.SizeAndDo(m)
    m, _ = state.Scrub(m)
    w.WriteMsg(m)
}

func Filter(request *request.Request, filterMode string, ips []handler.IP_Record) []handler.IP_Record {
    switch  filterMode {
    case "multi":
        return ips
    case "rr":
        return handler.GetWeightedIp(ips)
    case "geo_country":
        return g.GetSameCountry(GetSourceIp(request), ips)
    case "geo_location":
        return g.GetMinimumDistance(GetSourceIp(request), ips)
    default:
        log.Printf("[ERROR] invalid filter mode : %s", filterMode)
        return ips
    }
    return ips
}

func errorResponse(state request.Request, rcode int) {
    m := new(dns.Msg)
    m.SetRcode(state.Req, rcode)
    m.Authoritative, m.RecursionAvailable, m.Compress = true, false, true

    // m.Ns, _ = redis.SOA(state.Name(), zone, nil)

    state.SizeAndDo(m)
    state.W.WriteMsg(m)
}

func main() {
    configFile := "redins.ini"
    if len(os.Args) > 1 {
        configFile = os.Args[1]
    }
    cfg, err := ini.LooseLoad(configFile)
    if err != nil {
        log.Printf("[ERROR] loading config failed : %s", err)
        return
    }

    s = server.NewServer(server.LoadConfig(cfg, "server"))

    h = handler.NewHandler(handler.LoadConfig(cfg, "handler"))

    l = eventlog.NewLogger(eventlog.LoadConfig(cfg, "log"))

    g = geoip.NewGeoIp(geoip.LoadConfig(cfg, "geoip"))

    k = healthcheck.NewHealthcheck(healthcheck.LoadConfig(cfg, "healthcheck"))

    dns.HandleFunc(".", handleRequest)

    var wg sync.WaitGroup
    wg.Add(2)
    go s.ListenAndServe()
    go k.Start()
    wg.Wait()
}
