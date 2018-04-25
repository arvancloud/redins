package upstream

import (
    "testing"
    "log"
    "github.com/go-ini/ini"
    "github.com/miekg/dns"
)

func TestUpstream(t *testing.T) {
    cfg, err := ini.LooseLoad("test.ini")
    if err != nil {
        log.Printf("[ERROR] loading config failed : %s", err)
        t.Fail()
    }
    u := NewUpstream(LoadConfig(cfg,"upstream"))
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