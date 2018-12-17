package handler

import (
    "testing"
    "github.com/coredns/coredns/plugin/test"
    "github.com/miekg/dns"
    "github.com/coredns/coredns/plugin/pkg/dnstest"
    "github.com/coredns/coredns/request"
    "net"
    "log"
)

func TestSubnet(t *testing.T) {
    tc := test.Case {
        Qname: "example.com.", Qtype: dns.TypeA,

    }
    sa := "192.168.1.2"
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
    if r.IsEdns0() == nil {
        log.Printf("no edns\n")
        t.Fail()
    }
    w := dnstest.NewRecorder(&test.ResponseWriter{})
    state := request.Request{W: w, Req: r}

    subnet := GetSourceSubnet(&state)
    if subnet != sa + "/32/0" {
        log.Printf("subnet = %s should be %s\n", subnet, sa)
        t.Fail()
    }
    address := GetSourceIp(&state)
    if address.String() != sa {
        log.Printf("address = %s should be %s\n", address.String(), sa)
        t.Fail()
    }
}