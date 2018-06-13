package handler

import (
    "net"
    "testing"
    "log"

    "github.com/miekg/dns"
    "github.com/coredns/coredns/plugin/pkg/dnstest"
    "github.com/coredns/coredns/plugin/test"
    "github.com/coredns/coredns/request"
    "arvancloud/redins/config"
    "arvancloud/redins/dns_types"
    "arvancloud/redins/eventlog"
)

var lookupZones = []string {
    "example.com.", "example.net.", "example.aaa.",
}

var lookupEntries = [][][]string {
    {
        {"@",
            "{\"soa\":{\"ttl\":300, \"minttl\":100, \"mbox\":\"hostmaster.example.com.\",\"ns\":\"ns1.example.com.\",\"refresh\":44,\"retry\":55,\"expire\":66}," +
            "\"config\":{\"ip_filter_mode\":\"multi\", \"health_check\":{\"enable\":false}}}",
        },
        {"x",
            "{\"a\":[{\"ttl\":300, \"ip\":\"1.2.3.4\", \"country\":\"ES\"},{\"ttl\":300, \"ip\":\"5.6.7.8\", \"country\":\"\"}]," +
                "\"aaaa\":[{\"ttl\":300, \"ip\":\"::1\"}]," +
                "\"txt\":[{\"ttl\":300, \"text\":\"foo\"},{\"ttl\":300, \"text\":\"bar\"}]," +
                "\"ns\":[{\"ttl\":300, \"host\":\"ns1.example.com.\"},{\"ttl\":300, \"host\":\"ns2.example.com.\"}]," +
                "\"mx\":[{\"ttl\":300, \"host\":\"mx1.example.com.\", \"preference\":10},{\"ttl\":300, \"host\":\"mx2.example.com.\", \"preference\":10}]," +
                "\"srv\":[{\"ttl\":300, \"target\":\"sip.example.com.\",\"port\":555,\"priority\":10,\"weight\":100}]," +
                "\"config\":{\"ip_filter_mode\":\"multi\", \"health_check\":{\"enable\":false}}}",
        },
        {"y",
            "{\"cname\":{\"ttl\":300, \"host\":\"x.example.com.\"}," +
            "\"config\":{\"ip_filter_mode\":\"multi\", \"health_check\":{\"enable\":false}}}",
        },
        {"ns1",
            "{\"a\":[{\"ttl\":300, \"ip\":\"2.2.2.2\"}]," +
            "\"config\":{\"ip_filter_mode\":\"multi\", \"health_check\":{\"enable\":false}}}",
        },
        {"ns2",
            "{\"a\":[{\"ttl\":300, \"ip\":\"3.3.3.3\"}]," +
            "\"config\":{\"ip_filter_mode\":\"multi\", \"health_check\":{\"enable\":false}}}",
        },
        {"_sip._tcp",
            "{\"srv\":[{\"ttl\":300, \"target\":\"sip.example.com.\",\"port\":555,\"priority\":10,\"weight\":100}]," +
            "\"config\":{\"ip_filter_mode\":\"multi\", \"health_check\":{\"enable\":false}}}",
        },
        {"sip",
            "{\"a\":[{\"ttl\":300, \"ip\":\"7.7.7.7\"}]," +
            "\"aaaa\":[{\"ttl\":300, \"ip\":\"::1\"}]," +
            "\"config\":{\"ip_filter_mode\":\"multi\", \"health_check\":{\"enable\":false}}}",
        },
    },
    {
        {"@",
            "{\"soa\":{\"ttl\":300, \"minttl\":100, \"mbox\":\"hostmaster.example.net.\",\"ns\":\"ns1.example.net.\",\"refresh\":44,\"retry\":55,\"expire\":66}," +
            "\"ns\":[{\"ttl\":300, \"host\":\"ns1.example.net.\"},{\"ttl\":300, \"host\":\"ns2.example.net.\"}]," +
            "\"config\":{\"ip_filter_mode\":\"multi\", \"health_check\":{\"enable\":false}}}",
        },
        {"sub.*",
            "{\"txt\":[{\"ttl\":300, \"text\":\"this is not a wildcard\"}]," +
            "\"config\":{\"ip_filter_mode\":\"multi\", \"health_check\":{\"enable\":false}}}",
        },
        {"host1",
            "{\"a\":[{\"ttl\":300, \"ip\":\"5.5.5.5\"}]," +
            "\"config\":{\"ip_filter_mode\":\"multi\", \"health_check\":{\"enable\":false}}}",
        },
        {"subdel",
            "{\"ns\":[{\"ttl\":300, \"host\":\"ns1.subdel.example.net.\"},{\"ttl\":300, \"host\":\"ns2.subdel.example.net.\"}]," +
            "\"config\":{\"ip_filter_mode\":\"multi\", \"health_check\":{\"enable\":false}}}",
        },
        {"*",
            "{\"txt\":[{\"ttl\":300, \"text\":\"this is a wildcard\"}]," +
            "\"mx\":[{\"ttl\":300, \"host\":\"host1.example.net.\",\"preference\": 10}]," +
            "\"config\":{\"ip_filter_mode\":\"multi\", \"health_check\":{\"enable\":false}}}",
        },
        {"_ssh._tcp.host1",
            "{\"srv\":[{\"ttl\":300, \"target\":\"tcp.example.com.\",\"port\":123,\"priority\":10,\"weight\":100}]," +
            "\"config\":{\"ip_filter_mode\":\"multi\", \"health_check\":{\"enable\":false}}}",
        },
        {"_ssh._tcp.host2",
            "{\"srv\":[{\"ttl\":300, \"target\":\"tcp.example.com.\",\"port\":123,\"priority\":10,\"weight\":100}]," +
            "\"config\":{\"ip_filter_mode\":\"multi\", \"health_check\":{\"enable\":false}}}",
        },
    },
    {
        {"@",
            "{\"soa\":{\"ttl\":300, \"minttl\":100, \"mbox\":\"hostmaster.example.aaa.\",\"ns\":\"ns1.example.aaa.\",\"refresh\":44,\"retry\":55,\"expire\":66}," +
                "\"config\":{\"ip_filter_mode\":\"multi\", \"health_check\":{\"enable\":false}}}",
        },
        {"x",
            "{\"a\":[{\"ttl\":300, \"ip\":\"1.2.3.4\"}]," +
                "\"aaaa\":[{\"ttl\":300, \"ip\":\"::1\"}]," +
                "\"txt\":[{\"ttl\":300, \"text\":\"foo\"},{\"ttl\":300, \"text\":\"bar\"}]," +
                "\"ns\":[{\"ttl\":300, \"host\":\"ns1.example.aaa.\"},{\"ttl\":300, \"host\":\"ns2.example.aaa.\"}]," +
                "\"mx\":[{\"ttl\":300, \"host\":\"mx1.example.aaa.\", \"preference\":10},{\"ttl\":300, \"host\":\"mx2.example.aaa.\", \"preference\":10}]," +
                "\"srv\":[{\"ttl\":300, \"target\":\"sip.example.aaa.\",\"port\":555,\"priority\":10,\"weight\":100}]," +
                "\"config\":{\"ip_filter_mode\":\"multi\", \"health_check\":{\"enable\":false}}}",
        },
        {"y",
            "{\"cname\":{\"ttl\":300, \"host\":\"x.example.aaa.\"}," +
                "\"config\":{\"ip_filter_mode\":\"multi\", \"health_check\":{\"enable\":false}}}",
        },
        {"z",
            "{\"cname\":{\"ttl\":300, \"host\":\"y.example.aaa.\"}," +
                "\"config\":{\"ip_filter_mode\":\"multi\", \"health_check\":{\"enable\":false}}}",
        },
    },
}

var lookupTestCases = [][]test.Case{
    // basic tests
    {
        // A Test
        {
            Qname: "x.example.com.", Qtype: dns.TypeA,
            Answer: []dns.RR{
                test.A("x.example.com. 300 IN A 1.2.3.4"),
                test.A("x.example.com. 300 IN A 5.6.7.8"),
            },
        },
        // AAAA Test
        {
            Qname: "x.example.com.", Qtype: dns.TypeAAAA,
            Answer: []dns.RR{
                test.AAAA("x.example.com. 300 IN AAAA ::1"),
            },
        },
        // TXT Test
        {
            Qname: "x.example.com.", Qtype: dns.TypeTXT,
            Answer: []dns.RR{
                test.TXT("x.example.com. 300 IN TXT bar"),
                test.TXT("x.example.com. 300 IN TXT foo"),
            },
        },
        // CNAME Test
        {
            Qname: "y.example.com.", Qtype: dns.TypeCNAME,
            Answer: []dns.RR{
                test.CNAME("y.example.com. 300 IN CNAME x.example.com."),
            },
        },
        // NS Test
        {
            Qname: "x.example.com.", Qtype: dns.TypeNS,
            Answer: []dns.RR{
                test.NS("x.example.com. 300 IN NS ns1.example.com."),
                test.NS("x.example.com. 300 IN NS ns2.example.com."),
            },
        },
        // MX Test
        {
            Qname: "x.example.com.", Qtype: dns.TypeMX,
            Answer: []dns.RR{
                test.MX("x.example.com. 300 IN MX 10 mx1.example.com."),
                test.MX("x.example.com. 300 IN MX 10 mx2.example.com."),
            },
        },
        // SRV Test
        {
            Qname: "_sip._tcp.example.com.", Qtype: dns.TypeSRV,
            Answer: []dns.RR{
                test.SRV("_sip._tcp.example.com. 300 IN SRV 10 100 555 sip.example.com."),
            },
        },
        // NXDOMAIN Test
        {
            Qname: "notexists.example.com.", Qtype: dns.TypeA,
            Rcode: dns.RcodeNameError,
        },
        // SOA Test
        {
            Qname: "example.com.", Qtype: dns.TypeSOA,
            Answer: []dns.RR{
                test.SOA("example.com. 300 IN SOA ns1.example.com. hostmaster.example.com. 1460498836 44 55 66 100"),
            },
        },
    },
    // Wildcard Tests
    {
        {
            Qname: "host3.example.net.", Qtype: dns.TypeMX,
            Answer: []dns.RR{
                test.MX("host3.example.net. 300 IN MX 10 host1.example.net."),
            },
        },
        {
            Qname: "host3.example.net.", Qtype: dns.TypeA,
        },
        {
            Qname: "foo.bar.example.net.", Qtype: dns.TypeTXT,
            Answer: []dns.RR{
                test.TXT("foo.bar.example.net. 300 IN TXT \"this is a wildcard\""),
            },
        },
        {
            Qname: "host1.example.net.", Qtype: dns.TypeMX,
        },
        {
            Qname: "sub.*.example.net.", Qtype: dns.TypeMX,
        },
        {
            Qname: "host.subdel.example.net.", Qtype: dns.TypeA,
            Rcode: dns.RcodeNameError,
        },
        {
            Qname: "ghost.*.example.net.", Qtype: dns.TypeMX,
            Rcode: dns.RcodeNameError,
        },
        {
            Qname: "f.h.g.f.t.r.e.example.net.", Qtype: dns.TypeTXT,
            Answer: []dns.RR{
                test.TXT("f.h.g.f.t.r.e.example.net. 300 IN TXT \"this is a wildcard\""),
            },
        },
    },
    // CNAME tests
    {
        {
            Qname: "y.example.aaa.", Qtype: dns.TypeCNAME,
            Answer: []dns.RR{
                test.CNAME("y.example.aaa. 300 IN CNAME x.example.aaa."),
            },
        },
        {
            Qname: "z.example.aaa.", Qtype: dns.TypeCNAME,
            Answer: []dns.RR{
                test.CNAME("y.example.aaa. 300 IN CNAME x.example.aaa."),
                test.CNAME("z.example.aaa. 300 IN CNAME y.example.aaa."),
            },
        },
        {
            Qname: "z.example.aaa.", Qtype: dns.TypeA,
            Answer: []dns.RR{
                test.A("x.example.aaa. 300 IN A 1.2.3.4"),
                test.CNAME("y.example.aaa. 300 IN CNAME x.example.aaa."),
                test.CNAME("z.example.aaa. 300 IN CNAME y.example.aaa."),
            },
        },
    },
}

func TestLookup(t *testing.T) {
    cfg := config.LoadConfig("config.json")
    eventlog.Logger = eventlog.NewLogger(&cfg.ErrorLog)

    h := NewHandler(cfg)
    for i, zone := range lookupZones {
        h.Redis.Del(zone)
        for _, cmd := range lookupEntries[i] {
            err := h.Redis.HSet(zone, cmd[0], cmd[1])
            if err != nil {
                log.Printf("[ERROR] cannot connect to redis: %s", err)
                t.Fail()
            }
        }
        h.LoadZones()
        for _, tc := range lookupTestCases[i] {

            r := tc.Msg()
            w := dnstest.NewRecorder(&test.ResponseWriter{})
            state := request.Request{W: w, Req: r}
            h.HandleRequest(&state)

            resp := w.Msg

            test.SortAndCheck(t, resp, tc)
        }
    }

}

func TestWeight(t *testing.T) {
    cfg := config.LoadConfig("config.json")
    eventlog.Logger = eventlog.NewLogger(&cfg.ErrorLog)

    // distribution
    ips := []dns_types.IP_Record {
        { Ip:net.ParseIP("1.2.3.4"), Weight: 4},
        { Ip:net.ParseIP("2.3.4.5"), Weight: 1},
        { Ip:net.ParseIP("3.4.5.6"), Weight: 5},
        { Ip:net.ParseIP("4.5.6.7"), Weight: 10},
    }
    n := make([]int, 4)
    for i:= 0; i < 100000; i++ {
        x := GetWeightedIp(ips, map[string]interface{}{})
        switch x[0].Ip.String() {
        case "1.2.3.4": n[0]++
        case "2.3.4.5": n[1]++
        case "3.4.5.6": n[2]++
        case "4.5.6.7": n[3]++
        }
    }
    if n[0] > n[2] || n[2] > n[3] || n[1] > n[0] {
        t.Fail()
    }

    // all zero
    for i := range ips {
        ips[i].Weight = 0
    }
    n[0], n[1], n[2], n[3] = 0, 0, 0, 0
    for i:= 0; i < 100000; i++ {
        x := GetWeightedIp(ips, map[string]interface{}{})
        switch x[0].Ip.String() {
        case "1.2.3.4": n[0]++
        case "2.3.4.5": n[1]++
        case "3.4.5.6": n[2]++
        case "4.5.6.7": n[3]++
        }
    }
    for i := 0; i < 4; i++ {
        if n[i] < 2000 && n[i] > 3000 {
            t.Fail()
        }
    }

    // some zero
    n[0], n[1], n[2], n[3] = 0, 0, 0, 0
    ips[0].Weight, ips[1].Weight, ips[2].Weight, ips[3].Weight = 0, 5, 7, 0
    for i:= 0; i < 100000; i++ {
        x := GetWeightedIp(ips, map[string]interface{}{})
        switch x[0].Ip.String() {
        case "1.2.3.4": n[0]++
        case "2.3.4.5": n[1]++
        case "3.4.5.6": n[2]++
        case "4.5.6.7": n[3]++
        }
    }
    log.Println(n)
    if n[0] > 0 || n[3] > 0 {
        t.Fail()
    }
}

func TestMultiRR(t *testing.T) {
    cfg := config.LoadConfig("config.json")
    eventlog.Logger = eventlog.NewLogger(&cfg.ErrorLog)

    ips := []dns_types.IP_Record {
        { Ip:net.ParseIP("1.2.3.4"), Weight: 4},
        { Ip:net.ParseIP("2.3.4.5"), Weight: 1},
        { Ip:net.ParseIP("3.4.5.6"), Weight: 5},
        { Ip:net.ParseIP("4.5.6.7"), Weight: 10},
    }
    shuffled := ShuffleIps(ips)
    valid := false
    for i := range shuffled {
        if shuffled[i].Ip.String() != ips[i].Ip.String() {
            valid = true
            break
        }
    }
    if valid == false {
        t.Fail()
    }
}

var anameEntries = [][]string{
    {"@",
        "{\"soa\":{\"ttl\":300, \"minttl\":100, \"mbox\":\"hostmaster.arvancloud.com.\",\"ns\":\"ns1.example.com.\",\"refresh\":44,\"retry\":55,\"expire\":66}," +
            "\"aname\":{\"location\":\"arvancloud.com.\"}," +
            "\"config\":{\"ip_filter_mode\":\"multi\", \"health_check\":{\"enable\":false}}}",
    },
    {"www",
        "{\"a\":[{\"ttl\":300, \"ip\":\"1.2.3.4\", \"country\":\"ES\"},{\"ttl\":300, \"ip\":\"5.6.7.8\", \"country\":\"\"}]," +
            "\"aname\":{\"location\":\"www.arvancloud.com.\"}," +
            "\"config\":{\"ip_filter_mode\":\"multi\", \"health_check\":{\"enable\":false}}}",
    },
}

func TestANAME(t *testing.T) {
    zone := "arvancloud.com."
    cfg := config.LoadConfig("config.json")
    eventlog.Logger = eventlog.NewLogger(&cfg.ErrorLog)

    h := NewHandler(cfg)
    h.Redis.Del(zone)
    for _, cmd := range anameEntries {
        err := h.Redis.HSet(zone, cmd[0], cmd[1])
        if err != nil {
            log.Printf("[ERROR] cannot connect to redis: %s", err)
            t.Fail()
        }
    }
    h.LoadZones()
    z := h.LoadZone(zone)
    record := h.GetLocation(zone, z)
    answers, res := h.upstream.Query(record.ANAME.Location, dns.TypeA)
    log.Println(res)
    if res != dns.RcodeSuccess {
        t.Fail()
    }
    for _, a := range answers {
        log.Printf("%s\n", a.String())
    }
    record = h.GetLocation("www", z)
    answers, res = h.upstream.Query(record.ANAME.Location, dns.TypeA)
    log.Println(res)
    if res != dns.RcodeSuccess {
        t.Fail()
    }
    for _, a := range answers {
        log.Printf("%s\n", a.String())
    }
}

