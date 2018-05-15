package upstream

import (
    "testing"
    "log"
    "github.com/miekg/dns"
    "arvancloud/redins/config"
)

func TestUpstream(t *testing.T) {
    cfg := config.LoadConfig("config.json")
    u := NewUpstream(cfg)
    rs := u.Query("google.com.", dns.TypeAAAA)
    if len(rs) == 0 {
        log.Printf("[ERROR] AAAA failed")
        t.Fail()
    }
    rs = u.Query("google.com.", dns.TypeA)
    if len(rs) == 0 {
        log.Printf("[ERROR] A failed")
        t.Fail()
    }
    rs = u.Query("google.com.", dns.TypeTXT)
    if len(rs) == 0 {
        log.Printf("[ERROR] TXT failed")
        t.Fail()
    }
}