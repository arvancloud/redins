package handler

import (
    "crypto/rsa"
    "crypto/ecdsa"
    "errors"

    "github.com/miekg/dns"
    "arvancloud/redins/dns_types"
    "arvancloud/redins/eventlog"
)

func (h *DnsRequestHandler) SignLocation(record *dns_types.Record) {
    if len(record.A.Data) > 0 {
        a := h.A(record.Name, record, record.A.Data)
        if rrsig, err := Sign(a, record.Name, record.Zone, record.A.Ttl); err == nil {
            record.A.RRSig = rrsig
        } else {
            eventlog.Logger.Errorf("cannot sign a record set for %s : %s", record.Name+"."+record.Zone.Name, err)
        }
    }
    if len(record.AAAA.Data) > 0 {
        aaaa := h.AAAA(record.Name, record, record.AAAA.Data)
        if rrsig, err := Sign(aaaa, record.Name, record.Zone, record.AAAA.Ttl); err == nil {
            record.A.RRSig = rrsig
        } else {
            eventlog.Logger.Errorf("cannot sign aaaa record set for %s : %s", record.Name+"."+record.Zone.Name, err)
        }
    }
    if len(record.TXT.Data) > 0 {
        txt := h.TXT(record.Name, record)
        if rrsig, err := Sign(txt, record.Name, record.Zone, record.TXT.Ttl); err == nil {
            record.TXT.RRSig = rrsig
        } else {
            eventlog.Logger.Errorf("cannot sign txt record set for %s : %s", record.Name+"."+record.Zone.Name, err)
        }
    }
    if len(record.NS.Data) > 0 {
        ns := h.NS(record.Name, record)
        if rrsig, err := Sign(ns, record.Name, record.Zone, record.NS.Ttl); err == nil {
            record.NS.RRSig = rrsig
        } else {
            eventlog.Logger.Errorf("cannot sign ns record set for %s : %s", record.Name+"."+record.Zone.Name, err)
        }
    }
    if len(record.MX.Data) > 0 {
        mx := h.MX(record.Name, record)
        if rrsig, err := Sign(mx, record.Name, record.Zone, record.MX.Ttl); err == nil {
            record.MX.RRSig = rrsig
        } else {
            eventlog.Logger.Errorf("cannot sign mx record set for %s : %s", record.Name+"."+record.Zone.Name, err)
        }
    }
    if len(record.SRV.Data) > 0 {
        srv := h.SRV(record.Name, record)
        if rrsig, err := Sign(srv, record.Name, record.Zone, record.SRV.Ttl); err == nil {
            record.SRV.RRSig = rrsig
        } else {
            eventlog.Logger.Errorf("cannot sign srv record set for %s : %s", record.Name+"."+record.Zone.Name, err)
        }
    }
    if record.CNAME != nil {
        cname := h.CNAME(record.Name, record)
        if rrsig, err := Sign(cname, record.Name, record.Zone, record.CNAME.Ttl); err == nil {
            record.CNAME.RRSig = rrsig
        } else {
            eventlog.Logger.Errorf("cannot sign cname record set for %s : %s", record.Name+"'"+record.Zone.Name, err)
        }
    }
    if record.SOA != nil {
        soa := h.SOA(record.Name, record)
        if rrsig, err := Sign(soa, record.Name, record.Zone, record.SOA.Ttl); err == nil {
            record.SOA.RRSig = rrsig
        } else {
            eventlog.Logger.Errorf("cannot sign soa record set for %s : %s", record.Name+"'"+record.Zone.Name, err)
        }
    }
}

func Sign(rrs []dns.RR, name string, zone *dns_types.Zone, ttl uint32) (*dns.RRSIG, error) {
    rrsig := &dns.RRSIG {
        Hdr: dns.RR_Header { name, dns.TypeRRSIG, dns.ClassINET,ttl, 0},
        Inception:zone.KeyInception,
        Expiration:zone.KeyExpiration,
        KeyTag:zone.DnsKey.KeyTag(),
        SignerName:zone.DnsKey.Hdr.Name,
        Algorithm: zone.DnsKey.Algorithm,
    }
    switch rrsig.Algorithm {
    case dns.RSAMD5, dns.RSASHA1, dns.RSASHA1NSEC3SHA1, dns.RSASHA256, dns.RSASHA512:
        rrsig.Sign(zone.PrivateKey.(*rsa.PrivateKey), rrs)
    case dns.ECDSAP256SHA256, dns.ECDSAP384SHA384:
        rrsig.Sign(zone.PrivateKey.(*ecdsa.PrivateKey), rrs)
    case dns.DSA, dns.DSANSEC3SHA1:
        //rrsig.Sign(zone.PrivateKey.(*dsa.PrivateKey), rrs)
        fallthrough
    default:
        return nil, errors.New("invalid or not supported algorithm")
    }
    return rrsig, nil
}

func NSec(name string, zone *dns_types.Zone, ttl uint32) ([]dns.RR, error) {
    nsec := &dns.NSEC{
        Hdr: dns.RR_Header{name, dns.TypeNSEC, dns.ClassINET, ttl, 0},
        NextDomain: "\\000." + name,
        TypeBitMap: []uint16{dns.TypeRRSIG, dns.TypeNSEC},
    }
    sigs, err := Sign([]dns.RR{nsec}, name, zone, ttl)
    if err != nil {
        return nil, err
    }

    return []dns.RR{sigs, nsec}, nil
}
