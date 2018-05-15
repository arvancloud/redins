package main

import (
    "log"
    "sync"
    "strings"
    "os"
    "net"

    "github.com/miekg/dns"
    "github.com/coredns/coredns/request"
    "arvancloud/redins/handler"
    "arvancloud/redins/geoip"
    "arvancloud/redins/server"
    "arvancloud/redins/healthcheck"
    "arvancloud/redins/upstream"
    "arvancloud/redins/config"
)

var (
    s *dns.Server
    h *handler.DnsRequestHandler
    g *geoip.GeoIp
    k *healthcheck.Healthcheck
    u *upstream.Upstream
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

    data := RequestLogData {
        SourceIP: request.IP(),
        Record:   request.Name(),
    }

    opt := request.Req.IsEdns0()
    if opt != nil && len(opt.Option) != 0 {
        data.ClientSubnet = opt.Option[0].String()
    }
    h.Logger.Log(data, "request")
}

func handleRequest(w dns.ResponseWriter, r *dns.Msg) {
    log.Printf("[DEBUG] handle request")
    state := request.Request{W: w, Req: r}

    qname := state.Name()
    qtype := state.QType()

    log.Printf("[DEBUG] name : %s", state.Name())
    log.Printf("[DEBUG] type : %s", state.Type())

    LogRequest(&state)

    record, res := h.FetchRecord(qname)

    var answers []dns.RR

    if res == dns.RcodeSuccess {
        if qtype == dns.TypeA {
            ips := []handler.IP_Record{}
            if len(record.A) == 0 && record.ANAME != nil {
                answers = handler.GetANAME(record.ANAME.Location, record.ANAME.Proxy, dns.TypeA)
            } else {
                ips = append(ips, record.A...)
                ips = k.FilterHealthcheck(qname, record, ips)
                ips = Filter(&state, record.Config.IpFilterMode, ips)
                answers = h.A(qname, ips)
            }
        } else if qtype == dns.TypeAAAA {
            ips := []handler.IP_Record{}
            if len(record.AAAA) == 0 && record.ANAME != nil {
                answers = handler.GetANAME(record.ANAME.Location, record.ANAME.Proxy, dns.TypeAAAA)
            } else {
                ips = append(ips, record.AAAA...)
                ips = k.FilterHealthcheck(qname, record, ips)
                ips = Filter(&state, record.Config.IpFilterMode, ips)
                answers = h.AAAA(qname, ips)
            }
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
    } else if res == dns.RcodeNotAuth && u.Enable {
        answers = u.Query(qname, qtype)
    } else {
        errorResponse(state, res)
        return
    }

    m := new(dns.Msg)
    m.SetReply(r)
    m.Authoritative, m.RecursionAvailable, m.Compress = true, u.Enable, true

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
    configFile := "config.json"
    if len(os.Args) > 1 {
        configFile = os.Args[1]
    }
    cfg := config.LoadConfig(configFile)

    s = server.NewServer(cfg)

    h = handler.NewHandler(cfg)

    g = geoip.NewGeoIp(cfg)

    k = healthcheck.NewHealthcheck(cfg)

    u = upstream.NewUpstream(cfg)

    dns.HandleFunc(".", handleRequest)

    var wg sync.WaitGroup
    go s.ListenAndServe()
    go k.Start()
    wg.Add(2)
    wg.Wait()
}
