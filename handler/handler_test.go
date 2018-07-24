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
        {"@config",
            "{\"soa\":{\"ttl\":300, \"minttl\":100, \"mbox\":\"hostmaster.example.com.\",\"ns\":\"ns1.example.com.\",\"refresh\":44,\"retry\":55,\"expire\":66}}",
        },
        {"x",
            "{" +
            "\"a\":{\"ttl\":300, \"records\":[{\"ip\":\"1.2.3.4\", \"country\":\"ES\"},{\"ip\":\"5.6.7.8\", \"country\":\"\"}]}," +
            "\"aaaa\":{\"ttl\":300, \"records\":[{\"ip\":\"::1\"}]}," +
            "\"txt\":{\"ttl\":300, \"records\":[{\"text\":\"foo\"},{\"text\":\"bar\"}]}," +
            "\"ns\":{\"ttl\":300, \"records\":[{\"host\":\"ns1.example.com.\"},{\"host\":\"ns2.example.com.\"}]}," +
            "\"mx\":{\"ttl\":300, \"records\":[{\"host\":\"mx1.example.com.\", \"preference\":10},{\"host\":\"mx2.example.com.\", \"preference\":10}]}," +
            "\"srv\":{\"ttl\":300, \"records\":[{\"target\":\"sip.example.com.\",\"port\":555,\"priority\":10,\"weight\":100}]}" +
            "}",
        },
        {"y",
            "{\"cname\":{\"ttl\":300, \"host\":\"x.example.com.\"}}",
        },
        {"ns1",
            "{\"a\":{\"ttl\":300, \"records\":[{\"ip\":\"2.2.2.2\"}]}}",
        },
        {"ns2",
            "{\"a\":{\"ttl\":300, \"records\":[{\"ip\":\"3.3.3.3\"}]}}",
        },
        {"_sip._tcp",
            "{\"srv\":{\"ttl\":300, \"records\":[{\"target\":\"sip.example.com.\",\"port\":555,\"priority\":10,\"weight\":100}]}}",
        },
        {"sip",
            "{\"a\":{\"ttl\":300, \"records\":[{\"ip\":\"7.7.7.7\"}]}," +
            "{\"aaaa\":{\"ttl\":300, \"records\":[{\"ip\":\"::1\"}]}}",
        },
        {"t.u.v.w",
            "{\"a\":{\"ttl\":300, \"records\":[{\"ip\":\"9.9.9.9\"}]}}",
        },
    },
    {
        {"@config",
            "{\"soa\":{\"ttl\":300, \"minttl\":100, \"mbox\":\"hostmaster.example.net.\",\"ns\":\"ns1.example.net.\",\"refresh\":44,\"retry\":55,\"expire\":66}}",
        },
        {"@",
            "{\"ns\":{\"ttl\":300, \"records\":[{\"host\":\"ns1.example.net.\"},{\"host\":\"ns2.example.net.\"}]}}",
        },
        {"sub.*",
            "{\"txt\":{\"ttl\":300, \"records\":[{\"text\":\"this is not a wildcard\"}]}}",
        },
        {"host1",
            "{\"a\":{\"ttl\":300, \"records\":[{\"ip\":\"5.5.5.5\"}]}}",
        },
        {"subdel",
            "{\"ns\":{\"ttl\":300, \"records\":[{\"host\":\"ns1.subdel.example.net.\"},{\"host\":\"ns2.subdel.example.net.\"}]}}",
        },
        {"*",
            "{\"txt\":{\"ttl\":300, \"records\":[{\"text\":\"this is a wildcard\"}]}," +
            "\"mx\":{\"ttl\":300, \"records\":[{\"host\":\"host1.example.net.\",\"preference\": 10}]}}",
        },
        {"_ssh._tcp.host1",
            "{\"srv\":{\"ttl\":300, \"records\":[{\"target\":\"tcp.example.com.\",\"port\":123,\"priority\":10,\"weight\":100}]}}",
        },
        {"_ssh._tcp.host2",
            "{\"srv\":{\"ttl\":300, \"records\":[{\"target\":\"tcp.example.com.\",\"port\":123,\"priority\":10,\"weight\":100}]}}",
        },
    },
    {
        {"@config",
            "{\"soa\":{\"ttl\":300, \"minttl\":100, \"mbox\":\"hostmaster.example.aaa.\",\"ns\":\"ns1.example.aaa.\",\"refresh\":44,\"retry\":55,\"expire\":66}}",
        },
        {"x",
            "{\"a\":{\"ttl\":300, \"records\":[{\"ip\":\"1.2.3.4\"}]}," +
                "\"aaaa\":{\"ttl\":300, \"records\":[{\"ip\":\"::1\"}]}," +
                "\"txt\":{\"ttl\":300, \"records\":[{\"text\":\"foo\"},{\"ttl\":300, \"text\":\"bar\"}]}," +
                "\"ns\":{\"ttl\":300, \"records\":[{\"host\":\"ns1.example.aaa.\"},{\"ttl\":300, \"host\":\"ns2.example.aaa.\"}]}," +
                "\"mx\":{\"ttl\":300, \"records\":[{\"host\":\"mx1.example.aaa.\", \"preference\":10},{\"host\":\"mx2.example.aaa.\", \"preference\":10}]}," +
                "\"srv\":{\"ttl\":300, \"records\":[{\"target\":\"sip.example.aaa.\",\"port\":555,\"priority\":10,\"weight\":100}]}}",
        },
        {"y",
            "{\"cname\":{\"ttl\":300, \"host\":\"x.example.aaa.\"}}",
        },
        {"z",
            "{\"cname\":{\"ttl\":300, \"host\":\"y.example.aaa.\"}}",
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
            Ns: []dns.RR{
                test.SOA("example.com. 300 IN SOA ns1.example.com. hostmaster.example.com. 1460498836 44 55 66 100"),
            },
        },
        // SOA Test
        {
            Qname: "example.com.", Qtype: dns.TypeSOA,
            Answer: []dns.RR{
                test.SOA("example.com. 300 IN SOA ns1.example.com. hostmaster.example.com. 1460498836 44 55 66 100"),
            },
        },
        // Empty non-terminal Test
        // FIXME: should return NOERROR instead of NXDOMAIN
        /*
        {
            Qname:"v.w.example.com.", Qtype: dns.TypeA,
        },
        */
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
            Ns: []dns.RR{
                test.SOA("example.net. 300 IN SOA ns1.example.net. hostmaster.example.net. 1460498836 44 55 66 100"),
            },
        },
        {
            Qname: "foo.bar.example.net.", Qtype: dns.TypeTXT,
            Answer: []dns.RR{
                test.TXT("foo.bar.example.net. 300 IN TXT \"this is a wildcard\""),
            },
        },
        {
            Qname: "host1.example.net.", Qtype: dns.TypeMX,
            Ns: []dns.RR{
                test.SOA("example.net. 300 IN SOA ns1.example.net. hostmaster.example.net. 1460498836 44 55 66 100"),
            },
        },
        {
            Qname: "sub.*.example.net.", Qtype: dns.TypeMX,
            Ns: []dns.RR{
                test.SOA("example.net. 300 IN SOA ns1.example.net. hostmaster.example.net. 1460498836 44 55 66 100"),
            },
        },
        {
            Qname: "host.subdel.example.net.", Qtype: dns.TypeA,
            Rcode: dns.RcodeNameError,
            Ns: []dns.RR{
                test.SOA("example.net. 300 IN SOA ns1.example.net. hostmaster.example.net. 1460498836 44 55 66 100"),
            },
        },
        {
            Qname: "ghost.*.example.net.", Qtype: dns.TypeMX,
            Rcode: dns.RcodeNameError,
            Ns: []dns.RR{
                test.SOA("example.net. 300 IN SOA ns1.example.net. hostmaster.example.net. 1460498836 44 55 66 100"),
            },
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
    ips := []dns_types.IP_RR {
        { Ip:net.ParseIP("1.2.3.4"), Weight: 4},
        { Ip:net.ParseIP("2.3.4.5"), Weight: 1},
        { Ip:net.ParseIP("3.4.5.6"), Weight: 5},
        { Ip:net.ParseIP("4.5.6.7"), Weight: 10},
    }
    n := make([]int, 4)
    for i:= 0; i < 100000; i++ {
        x := ChooseIp(ips, true)
        switch ips[x].Ip.String() {
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
        x := ChooseIp(ips, true)
        switch ips[x].Ip.String() {
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
        x := ChooseIp(ips, true)
        switch ips[x].Ip.String() {
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


    // weighted = false
    n[0], n[1], n[2], n[3] = 0, 0, 0, 0
    ips[0].Weight, ips[1].Weight, ips[2].Weight, ips[3].Weight = 0, 5, 7, 0
    for i:= 0; i < 100000; i++ {
        x := ChooseIp(ips, false)
        switch ips[x].Ip.String() {
        case "1.2.3.4": n[0]++
        case "2.3.4.5": n[1]++
        case "3.4.5.6": n[2]++
        case "4.5.6.7": n[3]++
        }
    }
    log.Println(n)
    for i := 0; i < 4; i++ {
        if n[i] < 2000 && n[i] > 3000 {
            t.Fail()
        }
    }
}

var anameEntries = [][]string{
    {"@config",
        "{\"soa\":{\"ttl\":300, \"minttl\":100, \"mbox\":\"hostmaster.arvancloud.com.\",\"ns\":\"ns1.example.com.\",\"refresh\":44,\"retry\":55,\"expire\":66}}",
    },
    {"@",
        "{\"aname\":{\"location\":\"arvancloud.com.\"}}",
    },
    {"www",
        "{\"a\":{\"ttl\":300, \"records\":[{\"ip\":\"1.2.3.4\", \"country\":\"ES\"},{\"ip\":\"5.6.7.8\", \"country\":\"\"}]}," +
            "\"aname\":{\"location\":\"www.arvancloud.com.\"}}",
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
    record := h.LoadLocation(zone, z)
    answers, res := h.upstream.Query(record.ANAME.Location, dns.TypeA)
    log.Println(res)
    if res != dns.RcodeSuccess {
        t.Fail()
    }
    for _, a := range answers {
        log.Printf("%s\n", a.String())
    }
    record = h.LoadLocation("www", z)
    answers, res = h.upstream.Query(record.ANAME.Location, dns.TypeA)
    log.Println(res)
    if res != dns.RcodeSuccess {
        t.Fail()
    }
    for _, a := range answers {
        log.Printf("%s\n", a.String())
    }
}

var filterGeoEntries = [][]string{
    {"@config",
        "{\"soa\":{\"ttl\":300, \"minttl\":100, \"mbox\":\"hostmaster.filter.com.\",\"ns\":\"ns1.filter.com.\",\"refresh\":44,\"retry\":55,\"expire\":66}}",
    },
    {"ww1",
        "{\"a\":{\"ttl\":300, \"records\":[" +
            "{\"ip\":\"127.0.0.1\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.2\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.3\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.4\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.5\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.6\", \"country\":\"\"}]," +
            "\"filter\":{\"count\":\"multi\",\"order\":\"none\",\"geo_filter\":\"none\"}}}",
    },
    {"ww2",
        "{\"a\":{\"ttl\":300, \"records\":[" +
            "{\"ip\":\"127.0.0.1\", \"country\":\"US\"}," +
            "{\"ip\":\"127.0.0.2\", \"country\":\"GB\"}," +
            "{\"ip\":\"127.0.0.3\", \"country\":\"ES\"}," +
            "{\"ip\":\"127.0.0.4\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.5\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.6\", \"country\":\"\"}]," +
            "\"filter\":{\"count\":\"multi\",\"order\":\"none\",\"geo_filter\":\"country\"}}}",
    },
    {"ww3",
        "{\"a\":{\"ttl\":300, \"records\":[" +
            "{\"ip\":\"192.30.252.225\", \"country\":\"US\"}," +
            "{\"ip\":\"192.30.252.225\", \"country\":\"GB\"}," +
            "{\"ip\":\"192.30.252.225\", \"country\":\"ES\"}," +
            "{\"ip\":\"213.95.10.76\", \"country\":\"\"}," +
            "{\"ip\":\"213.95.10.76\", \"country\":\"\"}," +
            "{\"ip\":\"213.95.10.76\", \"country\":\"\"}]," +
            "\"filter\":{\"count\":\"multi\",\"order\":\"none\",\"geo_filter\":\"location\"}}}",
    },
}

var filterGeoTestCases = []test.Case{
    {
        Qname: "ww1.filtergeo.com.", Qtype: dns.TypeA,
        Answer: []dns.RR{
            test.A("ww1.filtergeo.com. 300 IN A 127.0.0.1"),
            test.A("ww1.filtergeo.com. 300 IN A 127.0.0.2"),
            test.A("ww1.filtergeo.com. 300 IN A 127.0.0.3"),
            test.A("ww1.filtergeo.com. 300 IN A 127.0.0.4"),
            test.A("ww1.filtergeo.com. 300 IN A 127.0.0.5"),
            test.A("ww1.filtergeo.com. 300 IN A 127.0.0.6"),
        },
    },
    {
        Qname: "ww2.filtergeo.com.", Qtype: dns.TypeA,
        Answer: []dns.RR{
            test.A("ww2.filtergeo.com. 300 IN A 127.0.0.4"),
            test.A("ww2.filtergeo.com. 300 IN A 127.0.0.5"),
            test.A("ww2.filtergeo.com. 300 IN A 127.0.0.6"),
        },
    },
    {
        Qname: "ww3.filtergeo.com.", Qtype: dns.TypeA,
        Answer: []dns.RR{
            test.A("ww3.filtergeo.com. 300 IN A 213.95.10.76"),
            test.A("ww3.filtergeo.com. 300 IN A 213.95.10.76"),
            test.A("ww3.filtergeo.com. 300 IN A 213.95.10.76"),
        },
    },
}

func TestGeoFilter(t *testing.T) {
    cfg := config.LoadConfig("config.json")
    eventlog.Logger = eventlog.NewLogger(&cfg.ErrorLog)

    zone := "filtergeo.com."
    h := NewHandler(cfg)
    h.Redis.Del(zone)
    for _, cmd := range filterGeoEntries {
        err := h.Redis.HSet(zone, cmd[0], cmd[1])
        if err != nil {
            log.Printf("[ERROR] cannot connect to redis: %s", err)
            t.Fail()
        }
    }
    h.LoadZones()
    for _, tc := range filterGeoTestCases {

        r := tc.Msg()
        w := dnstest.NewRecorder(&test.ResponseWriter{})
        state := request.Request{W: w, Req: r}
        h.HandleRequest(&state)

        resp := w.Msg

        test.SortAndCheck(t, resp, tc)
    }
}

var filterMultiEntries = [][]string{
    {"@config",
        "{\"soa\":{\"ttl\":300, \"minttl\":100, \"mbox\":\"hostmaster.filtermulti.com.\",\"ns\":\"ns1.filter.com.\",\"refresh\":44,\"retry\":55,\"expire\":66}}",
    },
    {"ww1",
        "{\"a\":{\"ttl\":300, \"records\":[" +
            "{\"ip\":\"127.0.0.1\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.2\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.3\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.4\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.5\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.6\", \"country\":\"\"}]," +
            "\"filter\":{\"count\":\"multi\",\"order\":\"none\",\"geo_filter\":\"none\"}}}",
    },
    {"ww2",
        "{\"a\":{\"ttl\":300, \"records\":[" +
            "{\"ip\":\"127.0.0.1\", \"country\":\"\", \"weight\":1}," +
            "{\"ip\":\"127.0.0.2\", \"country\":\"\", \"weight\":4}," +
            "{\"ip\":\"127.0.0.3\", \"country\":\"\", \"weight\":10}," +
            "{\"ip\":\"127.0.0.4\", \"country\":\"\", \"weight\":2}," +
            "{\"ip\":\"127.0.0.5\", \"country\":\"\", \"weight\":20}]," +
            "\"filter\":{\"count\":\"multi\",\"order\":\"weighted\",\"geo_filter\":\"none\"}}}",
    },
    {"ww3",
        "{\"a\":{\"ttl\":300, \"records\":[" +
            "{\"ip\":\"127.0.0.1\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.2\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.3\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.4\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.5\", \"country\":\"\"}]," +
            "\"filter\":{\"count\":\"multi\",\"order\":\"rr\",\"geo_filter\":\"none\"}}}",
    },
}

var filterMultiTestCases = []test.Case{
    {
        Qname: "ww1.filtermulti.com.", Qtype: dns.TypeA,
        Answer: []dns.RR{
            test.A("ww1.filtermulti.com. 300 IN A 127.0.0.1"),
            test.A("ww1.filtermulti.com. 300 IN A 127.0.0.2"),
            test.A("ww1.filtermulti.com. 300 IN A 127.0.0.3"),
            test.A("ww1.filtermulti.com. 300 IN A 127.0.0.4"),
            test.A("ww1.filtermulti.com. 300 IN A 127.0.0.5"),
            test.A("ww1.filtermulti.com. 300 IN A 127.0.0.6"),
        },
    },
    {
        Qname: "ww2.filtermulti.com.", Qtype: dns.TypeA,
        Answer: []dns.RR{
        },
    },
    {
        Qname: "ww3.filtermulti.com.", Qtype: dns.TypeA,
        Answer: []dns.RR{
        },
    },
}

func TestMultiFilter(t *testing.T) {
    cfg := config.LoadConfig("config.json")
    eventlog.Logger = eventlog.NewLogger(&cfg.ErrorLog)

    zone := "filtermulti.com."
    h := NewHandler(cfg)
    h.Redis.Del(zone)
    for _, cmd := range filterMultiEntries {
        err := h.Redis.HSet(zone, cmd[0], cmd[1])
        if err != nil {
            log.Printf("[ERROR] cannot connect to redis: %s", err)
            log.Println("1")
            t.Fail()
        }
    }
    h.LoadZones()

    for i := 0; i < 10; i++ {
        tc := filterMultiTestCases[0]
        r := tc.Msg()
        w := dnstest.NewRecorder(&test.ResponseWriter{})
        state := request.Request{W: w, Req: r}
        h.HandleRequest(&state)

        resp := w.Msg

        test.SortAndCheck(t, resp, tc)
    }

    w1, w4, w10, w2, w20 := 0, 0, 0, 0, 0
    for i := 0; i < 10000; i++ {
        tc := filterMultiTestCases[1]
        r := tc.Msg()
        w := dnstest.NewRecorder(&test.ResponseWriter{})
        state := request.Request{W: w, Req: r}
        h.HandleRequest(&state)

        resp := w.Msg
        if len(resp.Answer) != 5 {
            log.Println("2")
            t.Fail()
        }

        resa := resp.Answer[0].(*dns.A)

        switch resa.A.String() {
        case "127.0.0.1":
            w1++
        case "127.0.0.2":
            w4++
        case "127.0.0.3":
            w10++
        case "127.0.0.4":
            w2++
        case "127.0.0.5":
            w20++
        }
    }
    log.Println(w1, w2, w4, w10, w20)
    if w1 > w2 || w2 > w4 || w4 > w10 || w10 > w20 {
        log.Println("3")
        t.Fail()
    }

    rr := make([]int, 5)
    for i := 0; i < 10000; i++ {
        tc := filterMultiTestCases[2]
        r := tc.Msg()
        w := dnstest.NewRecorder(&test.ResponseWriter{})
        state := request.Request{W: w, Req: r}
        h.HandleRequest(&state)

        resp := w.Msg
        if len(resp.Answer) != 5 {
            log.Println("4")
            t.Fail()
        }

        resa := resp.Answer[0].(*dns.A)

        switch resa.A.String() {
        case "127.0.0.1":
            rr[0]++
        case "127.0.0.2":
            rr[1]++
        case "127.0.0.3":
            rr[2]++
        case "127.0.0.4":
            rr[3]++
        case "127.0.0.5":
            rr[4]++
        }
    }
    log.Println(rr)
    for i := range rr {
        if rr[i] < 1500 || rr[i] > 2500 {
            log.Println("5")
            t.Fail()
        }
    }
}

var filterSingleEntries = [][]string{
    {"@config",
        "{\"soa\":{\"ttl\":300, \"minttl\":100, \"mbox\":\"hostmaster.filtersingle.com.\",\"ns\":\"ns1.filter.com.\",\"refresh\":44,\"retry\":55,\"expire\":66}}",
    },
    {"ww1",
        "{\"a\":{\"ttl\":300, \"records\":[" +
            "{\"ip\":\"127.0.0.1\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.2\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.3\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.4\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.5\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.6\", \"country\":\"\"}]," +
            "\"filter\":{\"count\":\"single\",\"order\":\"none\",\"geo_filter\":\"none\"}}}",
    },
    {"ww2",
        "{\"a\":{\"ttl\":300, \"records\":[" +
            "{\"ip\":\"127.0.0.1\", \"country\":\"\", \"weight\":1}," +
            "{\"ip\":\"127.0.0.2\", \"country\":\"\", \"weight\":4}," +
            "{\"ip\":\"127.0.0.3\", \"country\":\"\", \"weight\":10}," +
            "{\"ip\":\"127.0.0.4\", \"country\":\"\", \"weight\":2}," +
            "{\"ip\":\"127.0.0.5\", \"country\":\"\", \"weight\":20}]," +
            "\"filter\":{\"count\":\"single\",\"order\":\"weighted\",\"geo_filter\":\"none\"}}}",
    },
    {"ww3",
        "{\"a\":{\"ttl\":300, \"records\":[" +
            "{\"ip\":\"127.0.0.1\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.2\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.3\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.4\", \"country\":\"\"}," +
            "{\"ip\":\"127.0.0.5\", \"country\":\"\"}]," +
            "\"filter\":{\"count\":\"single\",\"order\":\"rr\",\"geo_filter\":\"none\"}}}",
    },
}

var filterSingleTestCases = []test.Case{
    {
        Qname: "ww1.filtersingle.com.", Qtype: dns.TypeA,
        Answer: []dns.RR{
            test.A("ww1.filtersingle.com. 300 IN A 127.0.0.1"),
        },
    },
    {
        Qname: "ww2.filtersingle.com.", Qtype: dns.TypeA,
        Answer: []dns.RR{
        },
    },
    {
        Qname: "ww3.filtersingle.com.", Qtype: dns.TypeA,
        Answer: []dns.RR{
        },
    },
}

func TestSingleFilter(t *testing.T) {
    cfg := config.LoadConfig("config.json")
    eventlog.Logger = eventlog.NewLogger(&cfg.ErrorLog)

    zone := "filtersingle.com."
    h := NewHandler(cfg)
    h.Redis.Del(zone)
    for _, cmd := range filterSingleEntries {
        err := h.Redis.HSet(zone, cmd[0], cmd[1])
        if err != nil {
            log.Printf("[ERROR] cannot connect to redis: %s", err)
            log.Println("1")
            t.Fail()
        }
    }
    h.LoadZones()

    for i := 0; i < 10; i++ {
        tc := filterSingleTestCases[0]
        r := tc.Msg()
        w := dnstest.NewRecorder(&test.ResponseWriter{})
        state := request.Request{W: w, Req: r}
        h.HandleRequest(&state)

        resp := w.Msg

        test.SortAndCheck(t, resp, tc)
    }

    w1, w4, w10, w2, w20 := 0, 0, 0, 0, 0
    for i := 0; i < 10000; i++ {
        tc := filterSingleTestCases[1]
        r := tc.Msg()
        w := dnstest.NewRecorder(&test.ResponseWriter{})
        state := request.Request{W: w, Req: r}
        h.HandleRequest(&state)

        resp := w.Msg
        if len(resp.Answer) != 1 {
            log.Println("2")
            t.Fail()
        }

        resa := resp.Answer[0].(*dns.A)

        switch resa.A.String() {
        case "127.0.0.1":
            w1++
        case "127.0.0.2":
            w4++
        case "127.0.0.3":
            w10++
        case "127.0.0.4":
            w2++
        case "127.0.0.5":
            w20++
        }
    }
    log.Println(w1, w2, w4, w10, w20)
    if w1 > w2 || w2 > w4 || w4 > w10 || w10 > w20 {
        log.Println("3")
        t.Fail()
    }

    rr := make([]int, 5)
    for i := 0; i < 10000; i++ {
        tc := filterSingleTestCases[2]
        r := tc.Msg()
        w := dnstest.NewRecorder(&test.ResponseWriter{})
        state := request.Request{W: w, Req: r}
        h.HandleRequest(&state)

        resp := w.Msg
        if len(resp.Answer) != 1 {
            log.Println("4")
            t.Fail()
        }

        resa := resp.Answer[0].(*dns.A)

        switch resa.A.String() {
        case "127.0.0.1":
            rr[0]++
        case "127.0.0.2":
            rr[1]++
        case "127.0.0.3":
            rr[2]++
        case "127.0.0.4":
            rr[3]++
        case "127.0.0.5":
            rr[4]++
        }
    }
    log.Println(rr)
    for i := range rr {
        if rr[i] < 1500 || rr[i] > 2500 {
            log.Println("5")
            t.Fail()
        }
    }
}
