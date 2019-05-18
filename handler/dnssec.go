package handler

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"errors"

	"github.com/hawell/logger"
	"github.com/miekg/dns"
)

var (
	NSecTypes = []uint16{dns.TypeRRSIG, dns.TypeNSEC}
)

func Sign(rrs []dns.RR, name string, key *ZoneKey, ttl uint32) (*dns.RRSIG, error) {
	rrsig := &dns.RRSIG{
		Hdr:        dns.RR_Header{name, dns.TypeRRSIG, dns.ClassINET, ttl, 0},
		Inception:  key.KeyInception,
		Expiration: key.KeyExpiration,
		KeyTag:     key.DnsKey.KeyTag(),
		SignerName: key.DnsKey.Hdr.Name,
		Algorithm:  key.DnsKey.Algorithm,
	}
	switch rrsig.Algorithm {
	case dns.RSAMD5, dns.RSASHA1, dns.RSASHA1NSEC3SHA1, dns.RSASHA256, dns.RSASHA512:
		if err := rrsig.Sign(key.PrivateKey.(*rsa.PrivateKey), rrs); err != nil {
			logger.Default.Errorf("sign failed : %s", err)
			return nil, err
		}
	case dns.ECDSAP256SHA256, dns.ECDSAP384SHA384:
		if err := rrsig.Sign(key.PrivateKey.(*ecdsa.PrivateKey), rrs); err != nil {
			logger.Default.Errorf("sign failed : %s", err)
			return nil, err
		}
	case dns.DSA, dns.DSANSEC3SHA1:
		//rrsig.Sign(zone.PrivateKey.(*dsa.PrivateKey), rrs)
		fallthrough
	default:
		return nil, errors.New("invalid or not supported algorithm")
	}
	return rrsig, nil
}

func NSec(name string, zone *Zone) ([]dns.RR, error) {
	nsec := &dns.NSEC{
		Hdr:        dns.RR_Header{name, dns.TypeNSEC, dns.ClassINET, zone.Config.SOA.MinTtl, 0},
		NextDomain: "\\000." + name,
		TypeBitMap: NSecTypes,
	}
	sigs, err := Sign([]dns.RR{nsec}, name, zone.ZSK, zone.Config.SOA.MinTtl)
	if err != nil {
		return nil, err
	}

	return []dns.RR{nsec, sigs}, nil
}
