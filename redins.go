package main

import (
    "sync"
    "os"
    "time"

    "github.com/miekg/dns"
    "github.com/coredns/coredns/request"
    "arvancloud/redins/handler"
    "arvancloud/redins/server"
    "arvancloud/redins/config"
    "arvancloud/redins/eventlog"
)

var (
    s []dns.Server
    h *handler.DnsRequestHandler
)

func handleRequest(w dns.ResponseWriter, r *dns.Msg) {
    // log.Printf("[DEBUG] handle request")
    state := request.Request{W: w, Req: r}

    h.HandleRequest(&state)
}

func main() {
    configFile := "config.json"
    if len(os.Args) > 1 {
        configFile = os.Args[1]
    }
    cfg := config.LoadConfig(configFile)

    eventlog.Logger = eventlog.NewLogger(&cfg.ErrorLog)

    s = server.NewServer(cfg)

    h = handler.NewHandler(cfg)

    dns.HandleFunc(".", handleRequest)

    var wg sync.WaitGroup
    for i := range s {
        go s[i].ListenAndServe()
        wg.Add(1)
        time.Sleep(1 * time.Second)
    }
    wg.Wait()
}
