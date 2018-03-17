package main

import (
    "log"
    "sync"

    "github.com/miekg/dns"
    "github.com/go-ini/ini"
    "github.com/coredns/coredns/request"
    "github.com/hawell/redins/handler"
    "github.com/hawell/redins/eventlog"
    "github.com/hawell/redins/geoip"
    "github.com/hawell/redins/server"
    "github.com/hawell/redins/healthcheck"
)

var (
    s *dns.Server
    h *handler.DnsRequestHandler
    l *eventlog.EventLogger
    g *geoip.GeoIp
    k *healthcheck.Healthcheck

)

func GetSourceIp(request *request.Request) string {
    opt := request.Req.IsEdns0()
    if len(opt.Option) != 0 {
        return opt.Option[0].String()
    }
    return request.IP()
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
    if len(opt.Option) != 0 {
        data.ClientSubnet = opt.Option[0].String()
    }
    l.Log(data, "request")
}

func handleRequest(w dns.ResponseWriter, r *dns.Msg) {
    log.Printf("[INFO] handle request")
    state := request.Request{W: w, Req: r}

    qname := state.Name()
    qtype := state.Type()

    log.Printf("[INFO] name : %s", qname)
    log.Printf("[INFO] type : %s", qtype)

    LogRequest(&state)

    record, res := h.FetchRecord(qname)

    if res != dns.RcodeSuccess {
        errorResponse(state, res)
        return
    }

    g.FilterGeoIp(GetSourceIp(&state), record)

    k.FilterHealthcheck(qname, record)

    answers := make([]dns.RR, 0, 10)

    switch qtype {
    case "A":
        answers = h.A(qname, record)
    case "AAAA":
        answers = h.AAAA(qname, record)
    case "CNAME":
        answers = h.CNAME(qname, record)
    case "TXT":
        answers = h.TXT(qname, record)
    case "NS":
        answers = h.NS(qname, record)
    case "MX":
        answers = h.MX(qname, record)
    case "SRV":
        answers = h.SRV(qname, record)
    case "SOA":
        answers = h.SOA(qname, record)
    default:
        errorResponse(state, dns.RcodeNotImplemented)
        return
    }

    m := new(dns.Msg)
    m.SetReply(r)
    m.Authoritative, m.RecursionAvailable, m.Compress = true, false, true

    m.Answer = append(m.Answer, answers...)

    state.SizeAndDo(m)
    m, _ = state.Scrub(m)
    w.WriteMsg(m)
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
    cfg, err := ini.LooseLoad("redins.ini")
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
