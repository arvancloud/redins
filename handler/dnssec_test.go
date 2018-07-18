package handler

import (
    "testing"
    "arvancloud/redins/config"
    "arvancloud/redins/eventlog"
    "log"
    "github.com/coredns/coredns/plugin/test"
    "github.com/miekg/dns"
    "github.com/coredns/coredns/plugin/pkg/dnstest"
    "github.com/coredns/coredns/request"
    "sort"
)

var dnssecZone = string("dnssec_test.com.")

var dnssecEntries = [][]string {
    {"@",
        "{\"soa\":{\"ttl\":300, \"minttl\":100, \"mbox\":\"hostmaster.dnssec_test.com.\",\"ns\":\"ns1.dnssec_test.com.\",\"refresh\":44,\"retry\":55,\"expire\":66}}",
    },
    {"x",
        "{" +
            "\"a\":{\"ttl\":300, \"records\":[{\"ip\":\"1.2.3.4\", \"country\":\"ES\"},{\"ip\":\"5.6.7.8\", \"country\":\"\"}]}," +
            "\"aaaa\":{\"ttl\":300, \"records\":[{\"ip\":\"::1\"}]}," +
            "\"txt\":{\"ttl\":300, \"records\":[{\"text\":\"foo\"},{\"text\":\"bar\"}]}," +
            "\"ns\":{\"ttl\":300, \"records\":[{\"host\":\"ns1.dnssec_test.com.\"},{\"host\":\"ns2.dnssec_test.com.\"}]}," +
            "\"mx\":{\"ttl\":300, \"records\":[{\"host\":\"mx1.dnssec_test.com.\", \"preference\":10},{\"host\":\"mx2.dnssec_test.com.\", \"preference\":10}]}," +
            "\"srv\":{\"ttl\":300, \"records\":[{\"target\":\"sip.dnssec_test.com.\",\"port\":555,\"priority\":10,\"weight\":100}]}" +
            "}",
    },
    {"@config",
        "{\"dnssec\": true, \"cname_flattening\": true}",
    },
}

var dnssecKeyPriv = string(
`Private-key-format: v1.3
Algorithm: 5 (RSASHA1)
Modulus: oqwXm/EF8q6p5Rrj66Bbft+0Vk7Kj6TuvZp4nNl0htiT/8/92kIcri5gbxnV2v+p6jXYQI1Vx/vqP5cB0kPzjUQuJFVpm14fxOp89D6N0fPXR7xJ+SHs5nigHBIJdaP5
PublicExponent: AQAB
PrivateExponent: fJBa48aET3kAD7evn9aDOXwDk7Nx2NzrE7Uddr3tRPTDH7gdIuxNGfPZVDnsUG5EbX2JJf3JQsD7YXeQ+BGyytIi0ZTq8jsU63Np9hjheFx+IWSIz6S4JGnFKWRWUvuh
Prime1: 1c0EgZCXitPsdtEURwj1okEgzN9ze+QRP8adz0t+0s6ptB+bG3+YrhbzXcexE0tv
Prime2: wseiokM5ugXX0ZKy+8+lvumEZ94gM8Tyc031tFc1RRqIzB67k7139r/liNJoGXMX
Exponent1: WZyl79x3+CNdcGuv8RorQofDxLs/v0TXigCosnM1RAyFCs9Yhs0TZJyQAtWpPaoX
Exponent2: GXGcpBemBc/Xlm/UY6KHYz375tmUWU7j4P4RF6LAuasyrX9iP3Vjo18D6/CYWqK3
Coefficient: GhzOVUQcUJkvbYc9/+9MZngzDCeoetXDR6IILqG0/Rmt7FHWwSD7nOSoUUE5GslF
Created: 20180717134704
Publish: 20180717134704
Activate: 20180717134704
`)

var dnssecKeyPub = string("dnssec_test.com. IN DNSKEY 256 3 5 AwEAAaKsF5vxBfKuqeUa4+ugW37ftFZOyo+k7r2aeJzZdIbYk//P/dpC HK4uYG8Z1dr/qeo12ECNVcf76j+XAdJD841ELiRVaZteH8TqfPQ+jdHz 10e8Sfkh7OZ4oBwSCXWj+Q==")

var dnskeyQuery = test.Case {
    Do: true,
    Qname: "dnssec_test.com", Qtype: dns.TypeDNSKEY,
}

var dnssecTestCases = []test.Case{
    {
        Qname: "x.dnssec_test.com.", Qtype: dns.TypeA,
        Answer: []dns.RR{
            test.A("x.dnssec_test.com. 300 IN A 1.2.3.4"),
            test.A("x.dnssec_test.com. 300 IN A 5.6.7.8"),
            test.RRSIG("x.dnssec_test.com.	300	IN	RRSIG	A 5 3 300 20180726080503 20180718050503 22548 dnssec_test.com. b/rdGOnMQzKX4K9c3CLvJYb2ErFlrShy8vBh86Y28t1RRnN9OCj7L1AGhr+5xEge3mpuRNd2djXFh7CwZmAOm6R0/acRP1mw1RnlSANhaVt1Enr57c6+5grPgn7e45X3"),
        },
        Do: true,
        Extra: []dns.RR{
            test.OPT(4096, true),
        },
    },
    {
        Qname: "x.dnssec_test.com.", Qtype: dns.TypeAAAA,
        Answer: []dns.RR{
            test.AAAA("x.dnssec_test.com. 300 IN AAAA ::1"),
            test.RRSIG("x.dnssec_test.com.	300	IN	RRSIG	AAAA 5 3 300 20180726102716 20180718072716 22548 dnssec_test.com. Bl6GjbEY2jXyWhVuQzehQs4RVvrIRvLz72eXjvRKXTg6BGmcZF7CyZo1+R2w3p83gAA0yhs6UnSD/GMC5zmLeR5/8LiTzWa0S5f5xZNHwWNEUtrtnS7nGCCFDXfLUI3n"),
        },
        Do: true,
        Extra: []dns.RR{
            test.OPT(4096, true),
        },
    },
    // TXT Test
    {
        Qname: "x.dnssec_test.com.", Qtype: dns.TypeTXT,
        Answer: []dns.RR{
            test.TXT("x.dnssec_test.com. 300 IN TXT bar"),
            test.TXT("x.dnssec_test.com. 300 IN TXT foo"),
            test.RRSIG("x.dnssec_test.com.	300	IN	RRSIG	TXT 5 3 300 20180726102908 20180718072908 22548 dnssec_test.com. NND6mWXgQ1CY/KTsgPcjvty7FdLCFQdoHQ6Rmyv2hpPg12xTmAokB/TScTeL+zhvtt+9ktYnErspZc3LVoyPqZ8TYppHHoEXDR8OpyqmVcTPx/fzRuW5zmuUpofnhlV6"),
        },
        Do: true,
        Extra: []dns.RR{
            test.OPT(4096, true),
        },
    },
    // NS Test
    {
        Qname: "x.dnssec_test.com.", Qtype: dns.TypeNS,
        Answer: []dns.RR{
            test.NS("x.dnssec_test.com. 300 IN NS ns1.dnssec_test.com."),
            test.NS("x.dnssec_test.com. 300 IN NS ns2.dnssec_test.com."),
            test.RRSIG("x.dnssec_test.com.	300	IN	RRSIG	NS 5 3 300 20180726104727 20180718074727 22548 dnssec_test.com. NTYiqJBR8hFjYQcHeuUUWH2zIEqpF5xfFeHBb24icTbd5kg7VU9QHkzc/odnAFu80SfDJVnxX9OTV7re8Epp06CBT7m8VpUUv6+qnn6ma2qukWa8wyvFPg/PXJLA8cpG"),
        },
        Do: true,
        Extra: []dns.RR{
            test.OPT(4096, true),
        },
    },
    // MX Test
    {
        Qname: "x.dnssec_test.com.", Qtype: dns.TypeMX,
        Answer: []dns.RR{
            test.MX("x.dnssec_test.com. 300 IN MX 10 mx1.dnssec_test.com."),
            test.MX("x.dnssec_test.com. 300 IN MX 10 mx2.dnssec_test.com."),
            test.RRSIG("x.dnssec_test.com.	300	IN	RRSIG	MX 5 3 300 20180726104823 20180718074823 22548 dnssec_test.com. I0il28K7OmjA/hRwV/uPyieeg+EnpxRQmcUvZ1JsijIAqf6FVqDbysgrZfzZBheizMuLsEjPmmVTJrl34Y1ZEHxwD9oxgxWSDQ4L7kHLUeOSTRA73maHOtr+Sypygw6E"),
        },
        Do: true,
        Extra: []dns.RR{
            test.OPT(4096, true),
        },
    },
    // SRV Test
    {
        Qname: "x.dnssec_test.com.", Qtype: dns.TypeSRV,
        Answer: []dns.RR{
            test.SRV("x.dnssec_test.com. 300 IN SRV 10 100 555 sip.dnssec_test.com."),
            test.RRSIG("x.dnssec_test.com.	300	IN	RRSIG	SRV 5 3 300 20180726104916 20180718074916 22548 dnssec_test.com. hwyeNmMQ6K6Ja/ogepGQvGEyEiBeCd7Suhb6CL/uEREuREq1wcr9QhS2s3yKy9ZhjO9xs2x38vSSZHvRBvTjVxMIpPaQuxcWI02s/NgVLkRA5H0LpBPE5pyXDxTmtavV"),
        },
        Do: true,
        Extra: []dns.RR{
            test.OPT(4096, true),
        },
    },
}

func TestDNSSEC(t *testing.T) {
    cfg := config.LoadConfig("config.json")
    eventlog.Logger = eventlog.NewLogger(&cfg.ErrorLog)

    h := NewHandler(cfg)
    h.Redis.Del(dnssecZone)
    for _, cmd := range dnssecEntries {
        err := h.Redis.HSet(dnssecZone, cmd[0], cmd[1])
        if err != nil {
            log.Printf("[ERROR] cannot connect to redis: %s", err)
            t.Fail()
        }
    }
    h.Redis.Set(dnssecZone + "_pub", dnssecKeyPub)
    h.Redis.Set(dnssecZone + "_priv", dnssecKeyPriv)
    h.LoadZones()

    var dnskey dns.RR
    {
        r := dnskeyQuery.Msg()
        w := dnstest.NewRecorder(&test.ResponseWriter{})
        state := request.Request{W: w, Req: r}
        h.HandleRequest(&state)
        resp := w.Msg
        dnskey = resp.Answer[0]
    }

    for i, tc0 := range dnssecTestCases {
        tc := test.Case{
            Qname: dnssecTestCases[i].Qname, Qtype: dnssecTestCases[i].Qtype,
            Answer: make([]dns.RR, len(dnssecTestCases[i].Answer)),
            Do: true,
            Extra: []dns.RR{
                test.OPT(4096, true),
            },
        }
        copy(tc.Answer, dnssecTestCases[i].Answer)
        sort.Sort(test.RRSet(tc.Answer))

        r := tc.Msg()
        w := dnstest.NewRecorder(&test.ResponseWriter{})
        state := request.Request{W: w, Req: r}
        h.HandleRequest(&state)
        resp := w.Msg
        rrsig := resp.Answer[len(resp.Answer)-1].(*dns.RRSIG)
        if rrsig.Verify(dnskey.(*dns.DNSKEY), resp.Answer[0 : len(resp.Answer)-1]) != nil {
            t.Fail()
        }
        if rrsig.Verify(dnskey.(*dns.DNSKEY), tc0.Answer[0 : len(resp.Answer)-1]) != nil {
            t.Fail()
        }
        test.SortAndCheck(t, resp, tc)
    }
}