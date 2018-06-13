package upstream

import (
    "testing"
    /*
    "log"
    "github.com/miekg/dns"
    "arvancloud/redins/config"
    "arvancloud/redins/eventlog"
    */
    "arvancloud/redins/eventlog"
    "github.com/miekg/dns"
    "arvancloud/redins/config"
    "log"
)

func TestUpstream(t *testing.T) {
    cfg := config.LoadConfig("config.json")
    eventlog.Logger = eventlog.NewLogger(&cfg.ErrorLog)
    u := NewUpstream(cfg)
    rs, res := u.Query("google.com.", dns.TypeAAAA)
    if len(rs) == 0 || res != 0 {
        log.Printf("[ERROR] AAAA failed")
        t.Fail()
    }
    rs, res = u.Query("google.com.", dns.TypeA)
    if len(rs) == 0 || res != 0 {
        log.Printf("[ERROR] A failed")
        t.Fail()
    }
    rs, res = u.Query("google.com.", dns.TypeTXT)
    if len(rs) == 0 || res != 0 {
        log.Printf("[ERROR] TXT failed")
        t.Fail()
    }
}
