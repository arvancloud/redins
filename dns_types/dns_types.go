package dns_types

import (
    "net"
    "crypto"
    "github.com/miekg/dns"
)

type RRSets struct {
    A            IP_RRSet      `json:"a,omitempty"`
    AAAA         IP_RRSet      `json:"aaaa,omitempty"`
    TXT          TXT_RRSet     `json:"txt,omitempty"`
    CNAME        *CNAME_RRSet  `json:"cname,omitempty"`
    NS           NS_RRSet      `json:"ns,omitempty"`
    MX           MX_RRSet      `json:"mx,omitempty"`
    SRV          SRV_RRSet     `json:"srv,omitempty"`
    SOA          *SOA_RRSet    `json:"soa,omitempty"`
    ANAME        *ANAME_Record `json:"aname,omitempty"`
}

type Record struct {
    RRSets
    Zone *Zone  `json:"-"`
    Name string `json:"-"`
}

type ZoneKey struct {
    Algorithm uint8 `json:"algorithm,omitempty"`
    PublicKey string `json:"public_key,omitmpty"`
    PrivateKey string `json:"private_key,omitempty"`
}

type ZoneConfig struct {
    DnsSec   bool `json:"dnssec,omitempty"`
    CnameFlattening bool `json:"cname_flattening,omitempty"`
}

type Zone struct {
    Name      string
    Locations map[string]struct{}
    Config ZoneConfig
    DnsKey *dns.DNSKEY
    DnsKeySig dns.RR
    PrivateKey crypto.PrivateKey
    KeyInception uint32
    KeyExpiration uint32
}

type Base_RRSet struct {
    Ttl   uint32 `json:"ttl,omitempty"`
    RRSig dns.RR `json:"-"`
}

type IP_RRSet struct {
    Base_RRSet
    Data              []IP_RR             `json:"records,omitempty"`
    HealthCheckConfig IpHealthCheckConfig `json:"health_check,omitempty"`
    FilterConfig      IpFilterConfig      `json:"filter,omitempty"`
}

type IP_RR struct {
    Ip      net.IP `json:"ip"`
    Country string `json:"country,omitempty"`
    Weight  int    `json:"weight,omitempty"`
}

type IpHealthCheckConfig struct {
    Enable    bool          `json:"enable,omitempty"`
    Protocol  string        `json:"protocol,omitempty"`
    Uri       string        `json:"uri,omitempty"`
    Port      int           `json:"port,omitempty"`
    Timeout   int           `json:"timeout,omitempty"`
    UpCount   int           `json:"up_count,omitempty"`
    DownCount int           `json:"down_count,omitempty"`
}

type IpFilterConfig struct {
    Count     string `json:"count,omitempty"`      // "multi", "single"
    Order     string `json:"order,omitmpty"`       // "weighted", "rr", "none"
    GeoFilter string `json:"geo_filter,omitempty"` // "country", "location", "none"
}

type CNAME_RRSet struct {
    Base_RRSet
    Host string `json:"host"`
}

type TXT_RRSet struct {
    Base_RRSet
    Data []TXT_RR `json:"records,omitempty"`
}

type TXT_RR struct {
    Text string `json:"text"`
}

type NS_RRSet struct {
    Base_RRSet
    Data []NS_RR `json:"records,omitempty"`
}

type NS_RR struct {
    Host string `json:"host"`
}

type MX_RRSet struct {
    Base_RRSet
    Data []MX_RR `json:"records,omitempty"`
}

type MX_RR struct {
    Host       string `json:"host"`
    Preference uint16 `json:"preference"`
}

type SRV_RRSet struct {
    Base_RRSet
    Data []SRV_RR `json:"records,omitempty"`
}

type SRV_RR struct {
    Priority uint16 `json:"priority"`
    Weight   uint16 `json:"weight"`
    Port     uint16 `json:"port"`
    Target   string `json:"target"`
}

type SOA_RRSet struct {
    Base_RRSet
    Ns      string `json:"ns"`
    MBox    string `json:"MBox"`
    Refresh uint32 `json:"refresh"`
    Retry   uint32 `json:"retry"`
    Expire  uint32 `json:"expire"`
    MinTtl  uint32 `json:"minttl"`
}

type ANAME_Record struct {
    Location string `json:"location,omitempty"`
}

