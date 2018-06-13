package dns_types

import (
    "net"
)

type RRSet struct {
    A            []IP_Record   `json:"a,omitempty"`
    AAAA         []IP_Record   `json:"aaaa,omitempty"`
    TXT          []TXT_Record  `json:"txt,omitempty"`
    CNAME        CNAME_Record  `json:"cname,omitempty"`
    NS           []NS_Record   `json:"ns,omitempty"`
    MX           []MX_Record   `json:"mx,omitempty"`
    SRV          []SRV_Record  `json:"srv,omitempty"`
    SOA          SOA_Record    `json:"soa,omitempty"`
    ANAME        *ANAME_Record `json:"aname,omitempty"`
}

type HealthCheckRecordConfig struct {
    Enable    bool          `json:"enable,omitempty"`
    Protocol  string        `json:"protocol,omitempty"`
    Uri       string        `json:"uri,omitempty"`
    Port      int           `json:"port,omitempty"`
    Timeout   int           `json:"timeout,omitempty"`
    UpCount   int           `json:"up_count,omitempty"`
    DownCount int           `json:"down_count,omitempty"`
}

type RecordConfig struct {
    IpFilterMode string `json:"ip_filter_mode"` // "multi", "rr", "geo_country", "geo_location"
    HealthCheckConfig HealthCheckRecordConfig `json:"health_check"`
}

type Record struct {
    RRSet
    Config       RecordConfig   `json:"config,omitempty"`
    ZoneName     string         `json:"-"`
}

type IP_Record struct {
    Ttl         uint32 `json:"ttl,omitempty"`
    Ip          net.IP `json:"ip"`
    Country     string `json:"country,omitempty"`
    Weight      int    `json:"weight"`
}

type ANAME_Record struct {
    Location string `json:"location,omitempty"`
}

type TXT_Record struct {
    Ttl  uint32 `json:"ttl,omitempty"`
    Text string `json:"text"`
}

type CNAME_Record struct {
    Ttl  uint32 `json:"ttl,omitempty"`
    Host string `json:"host"`
}

type NS_Record struct {
    Ttl  uint32 `json:"ttl,omitempty"`
    Host string `json:"host"`
}

type MX_Record struct {
    Ttl        uint32 `json:"ttl,omitempty"`
    Host       string `json:"host"`
    Preference uint16 `json:"preference"`
}

type SRV_Record struct {
    Ttl      uint32 `json:"ttl,omitempty"`
    Priority uint16 `json:"priority"`
    Weight   uint16 `json:"weight"`
    Port     uint16 `json:"port"`
    Target   string `json:"target"`
}

type SOA_Record struct {
    Ttl     uint32 `json:"ttl,omitempty"`
    Ns      string `json:"ns"`
    MBox    string `json:"MBox"`
    Refresh uint32 `json:"refresh"`
    Retry   uint32 `json:"retry"`
    Expire  uint32 `json:"expire"`
    MinTtl  uint32 `json:"minttl"`
}

