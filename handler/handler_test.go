package handler

import (
    "net"
    "testing"
    "log"

    "github.com/miekg/dns"
    "github.com/coredns/coredns/plugin/pkg/dnstest"
    "github.com/coredns/coredns/plugin/test"
    "github.com/coredns/coredns/request"
    "github.com/hawell/logger"
    "github.com/hawell/uperdis"
)

var lookupZones = []string {
    "example.com.", "example.net.", "example.aaa.", "example.bbb.", "example.ccc."/*, "example.ddd."*//*, "example.caa."*/,
}

var lookupEntries = [][][]string {
    {
        {"@config",
            `{"soa":{"ttl":300, "minttl":100, "mbox":"hostmaster.example.com.","ns":"ns1.example.com.","refresh":44,"retry":55,"expire":66}}`,
        },
        {"x",
            `{
            "a":{"ttl":300, "records":[{"ip":"1.2.3.4", "country":"ES"},{"ip":"5.6.7.8", "country":""}]},
            "aaaa":{"ttl":300, "records":[{"ip":"::1"}]},
            "txt":{"ttl":300, "records":[{"text":"foo"},{"text":"bar"}]},
            "ns":{"ttl":300, "records":[{"host":"ns1.example.com."},{"host":"ns2.example.com."}]},
            "mx":{"ttl":300, "records":[{"host":"mx1.example.com.", "preference":10},{"host":"mx2.example.com.", "preference":10}]},
            "srv":{"ttl":300, "records":[{"target":"sip.example.com.","port":555,"priority":10,"weight":100}]}
            }`,
        },
        {"y",
            `{"cname":{"ttl":300, "host":"x.example.com."}}`,
        },
        {"ns1",
            `{"a":{"ttl":300, "records":[{"ip":"2.2.2.2"}]}}`,
        },
        {"ns2",
            `{"a":{"ttl":300, "records":[{"ip":"3.3.3.3"}]}}`,
        },
        {"_sip._tcp",
            `{"srv":{"ttl":300, "records":[{"target":"sip.example.com.","port":555,"priority":10,"weight":100}]}}`,
        },
        {"sip",
            `{"a":{"ttl":300, "records":[{"ip":"7.7.7.7"}]},
            "aaaa":{"ttl":300, "records":[{"ip":"::1"}]}}`,
        },
        {"t.u.v.w",
            `{"a":{"ttl":300, "records":[{"ip":"9.9.9.9"}]}}`,
        },
    },
    {
        {"@config",
            `{"soa":{"ttl":300, "minttl":100, "mbox":"hostmaster.example.net.","ns":"ns1.example.net.","refresh":44,"retry":55,"expire":66}}`,
        },
        {"@",
            `{"ns":{"ttl":300, "records":[{"host":"ns1.example.net."},{"host":"ns2.example.net."}]}}`,
        },
        {"sub.*",
            `{"txt":{"ttl":300, "records":[{"text":"this is not a wildcard"}]}}`,
        },
        {"host1",
            `{"a":{"ttl":300, "records":[{"ip":"5.5.5.5"}]}}`,
        },
        {"subdel",
            `{"ns":{"ttl":300, "records":[{"host":"ns1.subdel.example.net."},{"host":"ns2.subdel.example.net."}]}}`,
        },
        {"*",
            `{"txt":{"ttl":300, "records":[{"text":"this is a wildcard"}]},
            "mx":{"ttl":300, "records":[{"host":"host1.example.net.","preference": 10}]}}`,
        },
        {"_ssh._tcp.host1",
            `{"srv":{"ttl":300, "records":[{"target":"tcp.example.com.","port":123,"priority":10,"weight":100}]}}`,
        },
        {"_ssh._tcp.host2",
            `{"srv":{"ttl":300, "records":[{"target":"tcp.example.com.","port":123,"priority":10,"weight":100}]}}`,
        },
    },
    {
        {"@config",
            `{"soa":{"ttl":300, "minttl":100, "mbox":"hostmaster.example.aaa.","ns":"ns1.example.aaa.","refresh":44,"retry":55,"expire":66}}`,
        },
        {"x",
            `{"a":{"ttl":300, "records":[{"ip":"1.2.3.4"}]},
                "aaaa":{"ttl":300, "records":[{"ip":"::1"}]},
                "txt":{"ttl":300, "records":[{"text":"foo"},{"text":"bar"}]},
                "ns":{"ttl":300, "records":[{"host":"ns1.example.aaa."},{"ttl":300, "host":"ns2.example.aaa."}]},
                "mx":{"ttl":300, "records":[{"host":"mx1.example.aaa.", "preference":10},{"host":"mx2.example.aaa.", "preference":10}]},
                "srv":{"ttl":300, "records":[{"target":"sip.example.aaa.","port":555,"priority":10,"weight":100}]}}`,
        },
        {"y",
            `{"cname":{"ttl":300, "host":"x.example.aaa."}}`,
        },
        {"z",
            `{"cname":{"ttl":300, "host":"y.example.aaa."}}`,
        },
    },
    {
        {"@config",
            `{"soa":{"ttl":300, "minttl":100, "mbox":"hostmaster.example.bbb.","ns":"ns1.example.bbb.","refresh":44,"retry":55,"expire":66}}`,
        },
        {"x",
            `{"a":{"ttl":300, "records":[{"ip":"1.2.3.4"}]}}`,
        },
        {"y",
            `{"cname":{"ttl":300, "host":"x.example.bbb."}}`,
        },
        {"z",
            `{}`,
        },
    },
    {
        {"@config",
            `{"soa":{"ttl":300, "minttl":100, "mbox":"hostmaster.example.ccc.","ns":"ns1.example.ccc.","refresh":44,"retry":55,"expire":66}}`,
        },
        {"x",
            `{"txt":{"ttl":300, "records":[{"text":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}}`,
        },
    },
    /*
    {
        {"@config",
            `{"soa":{"ttl":300, "minttl":100, "mbox":"hostmaster.example.ddd.","ns":"ns1.example.ddd.","refresh":44,"retry":55,"expire":66},"cname_flattening":true}`,
        },
        {"a",
            `{"a":{"ttl":300, "records":[{"ip":"1.2.3.4"}]},
                "aaaa":{"ttl":300, "records":[{"ip":"::1"}]},
                "txt":{"ttl":300, "records":[{"text":"foo"},{"text":"bar"}]},
                "ns":{"ttl":300, "records":[{"host":"ns1.example.ddd."},{"ttl":300, "host":"ns2.example.ddd."}]},
                "mx":{"ttl":300, "records":[{"host":"mx1.example.ddd.", "preference":10},{"host":"mx2.example.ddd.", "preference":10}]},
                "srv":{"ttl":300, "records":[{"target":"sip.example.ddd.","port":555,"priority":10,"weight":100}]}}`,
        },
        {"b",
            `{"cname":{"ttl":300, "host":"a.example.ddd."}}`,
        },
        {"c",
            `{"cname":{"ttl":300, "host":"b.example.ddd."}}`,
        },
        {"d",
            `{"cname":{"ttl":300, "host":"c.example.ddd."}}`,
        },
        {"e",
            `{"cname":{"ttl":300, "host":"d.example.ddd."}}`,
        },
    },
    */
    /*
    {
        {"@config",
            `{"soa":{"ttl":300, "minttl":100, "mbox":"hostmaster.example.caa.","ns":"ns1.example.caa.","refresh":44,"retry":55,"expire":66}}`,
        },
        {"@",
            `{"caa":{"ttl":300, "records":[{"tag":"issue", "value":"godaddy.com;", "flag":0}]}}`,
        },
        {"a.b.c.d",
            `{"cname":{"ttl":300, "host":"b.c.d.example.caa."}}`,
        },
        {"b.c.d",
            `{"cname":{"ttl":300, "host":"c.d.example.caa."}}`,
        },
        {"c.d",
            `{"cname":{"ttl":300, "host":"d.example.caa."}}`,
        },
        {"d",
            `{"cname":{"ttl":300, "host":"x.y.z.example.caa."}}`,
        },
    },
    */
}

var lookupTestCases = [][]test.Case{
    // basic tests
    {
        // NOAUTH Test
        {
            Qname: "dsdsd.sdf.dfd.", Qtype: dns.TypeA,
            Rcode: dns.RcodeNotAuth,
        },
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
        // not implemented
        {
            Qname: "example.com.", Qtype: dns.TypeUNSPEC,
            Rcode: dns.RcodeNotImplemented,
            Ns: []dns.RR{
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
    // empty values tests
    {
        // empty A test
        {
            Qname: "z.example.bbb.", Qtype: dns.TypeA,
            Ns: []dns.RR{
                test.SOA("example.bbb. 300 IN SOA ns1.example.bbb. hostmaster.example.bbb. 1460498836 44 55 66 100"),
            },
        },
        // empty AAAA test
        {
            Qname: "z.example.bbb.", Qtype: dns.TypeAAAA,
            Ns: []dns.RR{
                test.SOA("example.bbb. 300 IN SOA ns1.example.bbb. hostmaster.example.bbb. 1460498836 44 55 66 100"),
            },
        },
        // empty TXT test
        {
            Qname: "z.example.bbb.", Qtype: dns.TypeTXT,
            Ns: []dns.RR{
                test.SOA("example.bbb. 300 IN SOA ns1.example.bbb. hostmaster.example.bbb. 1460498836 44 55 66 100"),
            },
        },
        // empty NS test
        {
            Qname: "z.example.bbb.", Qtype: dns.TypeNS,
            Ns: []dns.RR{
                test.SOA("example.bbb. 300 IN SOA ns1.example.bbb. hostmaster.example.bbb. 1460498836 44 55 66 100"),
            },
        },
        // empty MX test
        {
            Qname: "z.example.bbb.", Qtype: dns.TypeMX,
            Ns: []dns.RR{
                test.SOA("example.bbb. 300 IN SOA ns1.example.bbb. hostmaster.example.bbb. 1460498836 44 55 66 100"),
            },
        },
        // empty SRV test
        {
            Qname: "z.example.bbb.", Qtype: dns.TypeSRV,
            Ns: []dns.RR{
                test.SOA("example.bbb. 300 IN SOA ns1.example.bbb. hostmaster.example.bbb. 1460498836 44 55 66 100"),
            },
        },
        // empty CNAME test
        {
            Qname: "x.example.bbb.", Qtype: dns.TypeCNAME,
            Ns: []dns.RR{
                test.SOA("example.bbb. 300 IN SOA ns1.example.bbb. hostmaster.example.bbb. 1460498836 44 55 66 100"),
            },
        },
        // empty A test with cname
        {
            Qname: "y.example.bbb.", Qtype: dns.TypeA,
            Answer: []dns.RR{
                test.A("x.example.bbb.	300	IN	A	1.2.3.4"),
                test.CNAME("y.example.bbb.	300	IN	CNAME	x.example.bbb."),
            },
        },
        // empty AAAA test with cname
        {
            Qname: "y.example.bbb.", Qtype: dns.TypeAAAA,
            Answer: []dns.RR{
                test.CNAME("y.example.bbb.	300	IN	CNAME	x.example.bbb."),
            },
        },
        // empty TXT test with cname
        {
            Qname: "y.example.bbb.", Qtype: dns.TypeTXT,
            Answer: []dns.RR{
                test.CNAME("y.example.bbb.	300	IN	CNAME	x.example.bbb."),
            },
        },
        // empty NS test with cname
        {
            Qname: "y.example.bbb.", Qtype: dns.TypeNS,
            Answer: []dns.RR{
                test.CNAME("y.example.bbb.	300	IN	CNAME	x.example.bbb."),
            },
        },
        // empty MX test with cname
        {
            Qname: "y.example.bbb.", Qtype: dns.TypeMX,
            Answer: []dns.RR{
                test.CNAME("y.example.bbb.	300	IN	CNAME	x.example.bbb."),
            },
        },
        // empty SRV test with cname
        {
            Qname: "y.example.bbb.", Qtype: dns.TypeSRV,
            Answer: []dns.RR{
                test.CNAME("y.example.bbb.	300	IN	CNAME	x.example.bbb."),
            },
        },
    },
    // long text
    {
        {
            Qname: "x.example.ccc.", Qtype: dns.TypeTXT,
            Answer: []dns.RR{
                test.TXT("x.example.ccc. 300 IN TXT \"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\""),
            },
        },
    },
    // CNAME flattening
    /*
    {
        {
            Qname: "e.example.ddd.", Qtype: dns.TypeA,
            Answer: []dns.RR{
                test.A("e.example.ddd. 300 IN A 1.2.3.4"),
            },
        },
        {
            Qname: "e.example.ddd.", Qtype: dns.TypeAAAA,
            Answer: []dns.RR{
                test.AAAA("e.example.ddd. 300 IN AAAA ::1"),
            },
        },
        {
            Qname: "e.example.ddd.", Qtype: dns.TypeTXT,
            Answer: []dns.RR{
                test.TXT("e.example.ddd. 300 IN TXT \"bar\""),
                test.TXT("e.example.ddd. 300 IN TXT \"foo\""),
            },
        },
        {
            Qname: "e.example.ddd.", Qtype: dns.TypeNS,
            Answer: []dns.RR{
                test.NS("e.example.ddd. 300 IN NS ns1.example.ddd."),
                test.NS("e.example.ddd. 300 IN NS ns2.example.ddd."),
            },
        },
        // MX Test
        {
            Qname: "e.example.ddd.", Qtype: dns.TypeMX,
            Answer: []dns.RR{
                test.MX("e.example.ddd. 300 IN MX 10 mx1.example.ddd."),
                test.MX("e.example.ddd. 300 IN MX 10 mx2.example.ddd."),
            },
        },
        // SRV Test
        {
            Qname: "e.example.ddd.", Qtype: dns.TypeSRV,
            Answer: []dns.RR{
                test.SRV("e.example.ddd. 300 IN SRV 10 100 555 sip.example.ddd."),
            },
        },
        {
            Qname: "e.example.ddd.", Qtype: dns.TypeCNAME,
            Answer: []dns.RR{
                test.CNAME("e.example.ddd. 300 IN CNAME d.example.ddd."),
            },
        },
    },
    */
    // CAA Test
    /*
    {
        {
            Qname: "example.caa.", Qtype: dns.TypeCAA,
            Answer: []dns.RR{
                test.CAA("example.caa.	300	IN	CAA	0 issue \"godaddy.com;\""),
            },
        },
        {
            Qname: "a.b.c.d.example.caa", Qtype: dns.TypeCAA,
            Answer: []dns.RR{
                test.CAA("a.b.c.d.example.caa 300 IN CAA 0 issue \"godaddy.com;\""),
            },
        },
    },
    */
}

var handlerTestConfig = HandlerConfig {
    MaxTtl: 300,
    CacheTimeout: 60,
    ZoneReload: 600,
    Redis: uperdis.RedisConfig {
        Ip: "redis",
        Port: 6379,
        DB: 0,
        Password: "",
        Prefix: "test_",
        Suffix: "_test",
        ConnectTimeout: 0,
        ReadTimeout: 0,
    },
    Log: logger.LogConfig {
        Enable: false,
    },
    Upstream: []UpstreamConfig  {
        {
            Ip: "1.1.1.1",
            Port: 53,
            Protocol: "udp",
            Timeout: 1000,
        },
    },
    GeoIp: GeoIpConfig {
        Enable: true,
        CountryDB: "../geoCity.mmdb",
        ASNDB: "../geoIsp.mmdb",
    },
}

func TestLookup(t *testing.T) {
    logger.Default = logger.NewLogger(&logger.LogConfig{})

    h := NewHandler(&handlerTestConfig)
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

            if test.SortAndCheck(resp, tc) != nil {
                t.Fail()
            }
        }
    }

}

func TestWeight(t *testing.T) {
    logger.Default = logger.NewLogger(&logger.LogConfig{})

    // distribution
    ips := []IP_RR {
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

var anameEntries = [][][]string{
    {
        {"@config",
            `{"soa":{"ttl":300, "minttl":100, "mbox":"hostmaster.arvancloud.com.","ns":"ns1.arvancloud.com.","refresh":44,"retry":55,"expire":66}}`,
        },
        {"@",
            `{"aname":{"location":"aname.arvan.an."}}`,
        },
        {"upstream",
            `{"aname":{"location":"google.com."}}`,
        },
        {"upstream2",
            `{"aname":{"location":"aname2.arvan.an."}}`,
        },
    },
    {
        {"@config",
            `{"soa":{"ttl":300, "minttl":100, "mbox":"hostmaster.arvan.an.","ns":"ns1.arvan.an.","refresh":44,"retry":55,"expire":66}}`,
        },
        {"aname",
            `{"a":{"ttl":300, "records":[{"ip":"6.5.6.5"}]}, "aaaa":{"ttl":300, "records":[{"ip":"::1"}]}}`,
        },
        {"aname2",
            `{
            "a":{"ttl":300, "filter": {"count":"single", "order": "weighted", "geo_filter":"none"}, "records":[{"ip":"1.1.1.1", "weight":1},{"ip":"2.2.2.2", "weight":5},{"ip":"3.3.3.3", "weight":10}]},
            "aaaa":{"ttl":300, "filter": {"count":"single", "order": "weighted", "geo_filter":"none"}, "records":[{"ip":"2001:db8::1", "weight":1},{"ip":"2001:db8::2", "weight":5},{"ip":"2001:db8::3", "weight":10}]}
            }`,
        },
    },
}

var anameTestCases = []test.Case {
    {
        Qname: "arvancloud.com.", Qtype: dns.TypeA,
        Answer: []dns.RR {
            test.A("arvancloud.com. 300 IN A 6.5.6.5"),
        },
    },
    {
        Qname: "arvancloud.com.", Qtype: dns.TypeAAAA,
        Answer: []dns.RR {
            test.AAAA("arvancloud.com. 300 IN AAAA ::1"),
        },
    },
}

func TestANAME(t *testing.T) {
    zones := []string{"arvancloud.com.", "arvan.an."}
    logger.Default = logger.NewLogger(&logger.LogConfig{})

    h := NewHandler(&handlerTestConfig)
    for i, zone := range zones {
        h.Redis.Del(zone)
        for _, cmd := range anameEntries[i] {
            err := h.Redis.HSet(zone, cmd[0], cmd[1])
            if err != nil {
                log.Printf("[ERROR] cannot connect to redis: %s", err)
                t.Fail()
            }
        }
    }
    h.LoadZones()

    for _, tc := range anameTestCases {

        r := tc.Msg()
        w := dnstest.NewRecorder(&test.ResponseWriter{})
        state := request.Request{W: w, Req: r}
        h.HandleRequest(&state)

        resp := w.Msg

        if test.SortAndCheck(resp, tc) != nil {
            t.Fail()
        }
    }
}

func TestWeightedANAME(t *testing.T) {
    zones := []string{"arvancloud.com.", "arvan.an."}
    logger.Default = logger.NewLogger(&logger.LogConfig{})

    h := NewHandler(&handlerTestConfig)
    for i, zone := range zones {
        h.Redis.Del(zone)
        for _, cmd := range anameEntries[i] {
            err := h.Redis.HSet(zone, cmd[0], cmd[1])
            if err != nil {
                log.Printf("[ERROR] cannot connect to redis: %s", err)
                t.Fail()
            }
        }
    }
    h.LoadZones()

    tc := test.Case {
        Qname: "upstream2.arvancloud.com.", Qtype: dns.TypeA,
    }
    tc2 := test.Case {
        Qname: "upstream2.arvancloud.com.", Qtype: dns.TypeAAAA,
    }
    ip1 := 0
    ip2 := 0
    ip3 := 0
    for i := 0; i < 1000; i++ {
        r := tc.Msg()
        w := dnstest.NewRecorder(&test.ResponseWriter{})
        state := request.Request{W: w, Req: r}
        h.HandleRequest(&state)

        resp := w.Msg
        if resp.Rcode != dns.RcodeSuccess {
            t.Fail()
        }
        if len(resp.Answer) == 0 {
            t.Fail()
        }
        a := resp.Answer[0].(*dns.A)
        switch a.A.String() {
        case "1.1.1.1":ip1++
        case "2.2.2.2":ip2++
        case "3.3.3.3":ip3++
        default:
            t.Fail()
        }
    }
    if !(ip1<ip2 && ip2<ip3) {
        t.Fail()
    }
    ip61 := 0
    ip62 := 0
    ip63 := 0
    for i := 0; i < 1000; i++ {
        r := tc2.Msg()
        w := dnstest.NewRecorder(&test.ResponseWriter{})
        state := request.Request{W: w, Req: r}
        h.HandleRequest(&state)

        resp := w.Msg
        if resp.Rcode != dns.RcodeSuccess {
            t.Fail()
        }
        if len(resp.Answer) == 0 {
            t.Fail()
        }
        aaaa := resp.Answer[0].(*dns.AAAA)
        switch aaaa.AAAA.String() {
        case "2001:db8::1":ip61++
        case "2001:db8::2":ip62++
        case "2001:db8::3":ip63++
        default:
            t.Fail()
        }
    }
    if !(ip61<ip62 && ip62<ip63) {
        t.Fail()
    }
}

var filterGeoEntries = [][]string{
    {"@config",
        `{"soa":{"ttl":300, "minttl":100, "mbox":"hostmaster.filter.com.","ns":"ns1.filter.com.","refresh":44,"retry":55,"expire":66}}`,
    },
    {"ww1",
        `{"a":{"ttl":300, "records":[
            {"ip":"127.0.0.1", "country":""},
            {"ip":"127.0.0.2", "country":""},
            {"ip":"127.0.0.3", "country":""},
            {"ip":"127.0.0.4", "country":""},
            {"ip":"127.0.0.5", "country":""},
            {"ip":"127.0.0.6", "country":""}],
            "filter":{"count":"multi","order":"none","geo_filter":"none"}}}`,
    },
    {"ww2",
        `{"a":{"ttl":300, "records":[
            {"ip":"127.0.0.1", "country":"US"},
            {"ip":"127.0.0.2", "country":"GB"},
            {"ip":"127.0.0.3", "country":"ES"},
            {"ip":"127.0.0.4", "country":""},
            {"ip":"127.0.0.5", "country":""},
            {"ip":"127.0.0.6", "country":""}],
            "filter":{"count":"multi","order":"none","geo_filter":"country"}}}`,
    },
    {"ww3",
        `{"a":{"ttl":300, "records":[
            {"ip":"192.30.252.225"},
            {"ip":"94.76.229.204"},
            {"ip":"84.88.14.229"},
            {"ip":"192.168.0.1"}],
            "filter":{"count":"multi","order":"none","geo_filter":"location"}}}`,
    },
    {"ww4",
        `{"a":{"ttl":300, "records":[
            {"ip":"127.0.0.1", "asn":47447},
            {"ip":"127.0.0.2", "asn":20776},
            {"ip":"127.0.0.3", "asn":35470},
            {"ip":"127.0.0.4", "asn":0},
            {"ip":"127.0.0.5", "asn":0},
            {"ip":"127.0.0.6", "asn":0}],
        "filter":{"count":"multi", "order":"none","geo_filter":"asn"}}}`,
    },
    {"ww5",
        `{"a":{"ttl":300, "records":[
            {"ip":"127.0.0.1", "country":"DE", "asn":47447},
            {"ip":"127.0.0.2", "country":"DE", "asn":20776},
            {"ip":"127.0.0.3", "country":"DE", "asn":35470},
            {"ip":"127.0.0.4", "country":"GB", "asn":0},
            {"ip":"127.0.0.5", "country":"", "asn":0},
            {"ip":"127.0.0.6", "country":"", "asn":0}],
        "filter":{"count":"multi", "order":"none","geo_filter":"asn+country"}}}`,
    },
    {"ww6",
        `{"a":{"ttl":300, "records":[
            {"ip":"127.0.0.1", "asn":[47447,20776]},
            {"ip":"127.0.0.2", "asn":[0,35470]},
            {"ip":"127.0.0.3", "asn":35470},
            {"ip":"127.0.0.4", "asn":0},
            {"ip":"127.0.0.5", "asn":[]},
            {"ip":"127.0.0.6"}],
        "filter":{"count":"multi", "order":"none","geo_filter":"asn"}}}`,
    },
    {"ww7",
        `{"a":{"ttl":300, "records":[
            {"ip":"127.0.0.1", "country":["DE", "GB"]},
            {"ip":"127.0.0.2", "country":["", "DE"]},
            {"ip":"127.0.0.3", "country":"DE"},
            {"ip":"127.0.0.4", "country":"CA"},
            {"ip":"127.0.0.5", "country": ""},
            {"ip":"127.0.0.6", "country": []},
            {"ip":"127.0.0.7"}],
        "filter":{"count":"multi", "order":"none","geo_filter":"country"}}}`,
    },
}

var filterGeoSourceIps = []string {
    "127.0.0.1",
    "127.0.0.1",
    "127.0.0.1",
    "127.0.0.1",
    "127.0.0.1",
    "94.76.229.204", // country = GB
    "154.11.253.242", // location = CA near US
    "212.83.32.45", // ASN = 47447
    "212.83.32.45", // country = DE, ASN = 47447
    "212.83.32.45",
    "178.18.89.144",
    "127.0.0.1",
    "213.95.10.76", // DE
    "94.76.229.204", // GB
    "154.11.253.242", // CA
    "127.0.0.1",
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
            test.A("ww3.filtergeo.com. 300 IN A 192.168.0.1"),
        },
    },
    {
        Qname: "ww4.filtergeo.com.", Qtype: dns.TypeA,
        Answer: []dns.RR{
            test.A("ww4.filtergeo.com. 300 IN A 127.0.0.4"),
            test.A("ww4.filtergeo.com. 300 IN A 127.0.0.5"),
            test.A("ww4.filtergeo.com. 300 IN A 127.0.0.6"),
        },
    },
    {
        Qname: "ww5.filtergeo.com.", Qtype: dns.TypeA,
        Answer: []dns.RR{
            test.A("ww5.filtergeo.com. 300 IN A 127.0.0.5"),
            test.A("ww5.filtergeo.com. 300 IN A 127.0.0.6"),
        },
    },
    {
        Qname: "ww2.filtergeo.com.", Qtype: dns.TypeA,
        Answer: []dns.RR{
            test.A("ww2.filtergeo.com. 300 IN A 127.0.0.2"),
        },
    },
    {
        Qname: "ww3.filtergeo.com.", Qtype: dns.TypeA,
        Answer: []dns.RR{
            test.A("ww3.filtergeo.com. 300 IN A 192.30.252.225"),
        },
    },
    {
        Qname: "ww4.filtergeo.com.", Qtype: dns.TypeA,
        Answer: []dns.RR{
            test.A("ww4.filtergeo.com. 300 IN A 127.0.0.1"),
        },
    },
    {
        Qname: "ww5.filtergeo.com.", Qtype: dns.TypeA,
        Answer: []dns.RR{
            test.A("ww5.filtergeo.com. 300 IN A 127.0.0.1"),
        },
    },
    {
        Qname: "ww6.filtergeo.com.", Qtype: dns.TypeA,
        Answer: []dns.RR{
            test.A("ww6.filtergeo.com. 300 IN A 127.0.0.1"),
        },
    },
    {
        Qname: "ww6.filtergeo.com.", Qtype: dns.TypeA,
        Answer: []dns.RR{
            test.A("ww6.filtergeo.com. 300 IN A 127.0.0.2"),
            test.A("ww6.filtergeo.com. 300 IN A 127.0.0.3"),
        },
    },
    {
        Qname: "ww6.filtergeo.com.", Qtype: dns.TypeA,
        Answer: []dns.RR{
            test.A("ww6.filtergeo.com. 300 IN A 127.0.0.2"),
            test.A("ww6.filtergeo.com. 300 IN A 127.0.0.4"),
            test.A("ww6.filtergeo.com. 300 IN A 127.0.0.5"),
            test.A("ww6.filtergeo.com. 300 IN A 127.0.0.6"),
        },
    },
    {
        Qname: "ww7.filtergeo.com.", Qtype: dns.TypeA,
        Answer: []dns.RR{
            test.A("ww7.filtergeo.com. 300 IN A 127.0.0.1"),
            test.A("ww7.filtergeo.com. 300 IN A 127.0.0.2"),
            test.A("ww7.filtergeo.com. 300 IN A 127.0.0.3"),
        },
    },
    {
        Qname: "ww7.filtergeo.com.", Qtype: dns.TypeA,
        Answer: []dns.RR{
            test.A("ww7.filtergeo.com. 300 IN A 127.0.0.1"),
        },
    },
    {
        Qname: "ww7.filtergeo.com.", Qtype: dns.TypeA,
        Answer: []dns.RR{
            test.A("ww7.filtergeo.com. 300 IN A 127.0.0.4"),
        },
    },
    {
        Qname: "ww7.filtergeo.com.", Qtype: dns.TypeA,
        Answer: []dns.RR{
            test.A("ww7.filtergeo.com. 300 IN A 127.0.0.2"),
            test.A("ww7.filtergeo.com. 300 IN A 127.0.0.5"),
            test.A("ww7.filtergeo.com. 300 IN A 127.0.0.6"),
            test.A("ww7.filtergeo.com. 300 IN A 127.0.0.7"),
        },
    },

}

func TestGeoFilter(t *testing.T) {
    logger.Default = logger.NewLogger(&logger.LogConfig{Target:"file", Enable:true, Path:"/tmp/rtest.log", Format:"txt"})

    zone := "filtergeo.com."
    h := NewHandler(&handlerTestConfig)
    h.Redis.Del(zone)
    for _, cmd := range filterGeoEntries {
        err := h.Redis.HSet(zone, cmd[0], cmd[1])
        if err != nil {
            log.Printf("[ERROR] cannot connect to redis: %s", err)
            t.Fail()
        }
    }
    h.LoadZones()
    for i, tc := range filterGeoTestCases {
        sa := filterGeoSourceIps[i]
        opt := &dns.OPT {
            Hdr: dns.RR_Header{Name:".", Rrtype:dns.TypeOPT,Class:dns.ClassANY, Rdlength:0, Ttl: 300,},
            Option: []dns.EDNS0 {
                &dns.EDNS0_SUBNET{
                    Address:net.ParseIP(sa),
                    Code:dns.EDNS0SUBNET,
                    Family: 1,
                    SourceNetmask:32,
                    SourceScope:0,
                },
            },
        }
        r := tc.Msg()
        r.Extra = append(r.Extra, opt)
        w := dnstest.NewRecorder(&test.ResponseWriter{})
        state := request.Request{W: w, Req: r}
        h.HandleRequest(&state)

        resp := w.Msg
        resp.Extra = nil

        if test.SortAndCheck(resp, tc) != nil {
            t.Fail()
        }
    }
}

var filterMultiEntries = [][]string{
    {"@config",
        `{"soa":{"ttl":300, "minttl":100, "mbox":"hostmaster.filtermulti.com.","ns":"ns1.filter.com.","refresh":44,"retry":55,"expire":66}}`,
    },
    {"ww1",
        `{"a":{"ttl":300, "records":[
            {"ip":"127.0.0.1", "country":""},
            {"ip":"127.0.0.2", "country":""},
            {"ip":"127.0.0.3", "country":""},
            {"ip":"127.0.0.4", "country":""},
            {"ip":"127.0.0.5", "country":""},
            {"ip":"127.0.0.6", "country":""}],
            "filter":{"count":"multi","order":"none","geo_filter":"none"}}}`,
    },
    {"ww2",
        `{"a":{"ttl":300, "records":[
            {"ip":"127.0.0.1", "country":"", "weight":1},
            {"ip":"127.0.0.2", "country":"", "weight":4},
            {"ip":"127.0.0.3", "country":"", "weight":10},
            {"ip":"127.0.0.4", "country":"", "weight":2},
            {"ip":"127.0.0.5", "country":"", "weight":20}],
            "filter":{"count":"multi","order":"weighted","geo_filter":"none"}}}`,
    },
    {"ww3",
        `{"a":{"ttl":300, "records":[
            {"ip":"127.0.0.1", "country":""},
            {"ip":"127.0.0.2", "country":""},
            {"ip":"127.0.0.3", "country":""},
            {"ip":"127.0.0.4", "country":""},
            {"ip":"127.0.0.5", "country":""}],
            "filter":{"count":"multi","order":"rr","geo_filter":"none"}}}`,
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
    logger.Default = logger.NewLogger(&logger.LogConfig{})

    zone := "filtermulti.com."
    h := NewHandler(&handlerTestConfig)
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

        if test.SortAndCheck(resp, tc) != nil {
            t.Fail()
        }
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
        `{"soa":{"ttl":300, "minttl":100, "mbox":"hostmaster.filtersingle.com.","ns":"ns1.filter.com.","refresh":44,"retry":55,"expire":66}}`,
    },
    {"ww1",
        `{"a":{"ttl":300, "records":[
            {"ip":"127.0.0.1", "country":""},
            {"ip":"127.0.0.2", "country":""},
            {"ip":"127.0.0.3", "country":""},
            {"ip":"127.0.0.4", "country":""},
            {"ip":"127.0.0.5", "country":""},
            {"ip":"127.0.0.6", "country":""}],
            "filter":{"count":"single","order":"none","geo_filter":"none"}}}`,
    },
    {"ww2",
        `{"a":{"ttl":300, "records":[
            {"ip":"127.0.0.1", "country":"", "weight":1},
            {"ip":"127.0.0.2", "country":"", "weight":4},
            {"ip":"127.0.0.3", "country":"", "weight":10},
            {"ip":"127.0.0.4", "country":"", "weight":2},
            {"ip":"127.0.0.5", "country":"", "weight":20}],
            "filter":{"count":"single","order":"weighted","geo_filter":"none"}}}`,
    },
    {"ww3",
        `{"a":{"ttl":300, "records":[
            {"ip":"127.0.0.1", "country":""},
            {"ip":"127.0.0.2", "country":""},
            {"ip":"127.0.0.3", "country":""},
            {"ip":"127.0.0.4", "country":""},
            {"ip":"127.0.0.5", "country":""}],
            "filter":{"count":"single","order":"rr","geo_filter":"none"}}}`,
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
    logger.Default = logger.NewLogger(&logger.LogConfig{})

    zone := "filtersingle.com."
    h := NewHandler(&handlerTestConfig)
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

        if test.SortAndCheck(resp, tc) != nil {
            t.Fail()
        }
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


var upstreamCNAME = [][]string {
    {"@config",
        `{"soa":{"ttl":300, "minttl":100, "mbox":"hostmaster.upstreamcname.com.","ns":"ns1.upstreamcname.com.","refresh":44,"retry":55,"expire":66}}`,
    },
    {"upstream",
        `{"cname":{"ttl":300, "host":"www.google.com"}}`,
    },
}

var upstreamCNAMETestCases = []test.Case{
    {
        Qname: "upstream.upstreamcname.com.", Qtype: dns.TypeA,
    },
}

func TestUpstreamCNAME(t *testing.T) {
    logger.Default = logger.NewLogger(&logger.LogConfig{})

    zone := "upstreamcname.com."
    h := NewHandler(&handlerTestConfig)
    h.Redis.Del(zone)
    for _, cmd := range upstreamCNAME {
        err := h.Redis.HSet(zone, cmd[0], cmd[1])
        if err != nil {
            log.Printf("[ERROR] cannot connect to redis: %s", err)
            log.Println("1")
            t.Fail()
        }
    }
    h.LoadZones()

    h.Config.UpstreamFallback = false
    {
        tc := upstreamCNAMETestCases[0]
        r := tc.Msg()
        w := dnstest.NewRecorder(&test.ResponseWriter{})
        state := request.Request{W: w, Req: r}
        h.HandleRequest(&state)

        resp := w.Msg
        log.Println(resp)
        if resp.Rcode != dns.RcodeSuccess {
            log.Println("1")
            t.Fail()
        }
        cname := resp.Answer[0].(*dns.CNAME)
        if cname.Target != "www.google.com." {
            log.Println("2 ", cname)
            t.Fail()
        }
    }

    h.Config.UpstreamFallback = true
    {
        tc := upstreamCNAMETestCases[0]
        r := tc.Msg()
        w := dnstest.NewRecorder(&test.ResponseWriter{})
        state := request.Request{W: w, Req: r}
        h.HandleRequest(&state)

        resp := w.Msg
        log.Println(resp)
        if resp.Rcode != dns.RcodeSuccess {
            log.Println("3")
            t.Fail()
        }
        hasCNAME := false
        hasA := false
        for _, rr := range resp.Answer {
            switch rr.(type) {
            case *dns.CNAME:
                hasCNAME = true
            case *dns.A:
                hasA = true
            }
        }
        if !hasCNAME || !hasA {
            log.Println("4")
            t.Fail()
        }
    }
}