package handler

import (
    "testing"
    "net"
    "log"

    "github.com/hawell/logger"
)

func TestGeoIpAutomatic(t *testing.T) {
    sip := [][]string {
        {"212.83.32.45", "DE", "213.95.10.76"},
        {"80.67.163.250", "FR", "62.240.228.4"},
        {"178.18.89.144", "NL", "46.19.36.12"},
        {"206.108.0.43", "CA", "154.11.253.242"},
        {"185.70.144.117", "DE", "213.95.10.76"},
        {"62.220.128.73", "CH", "82.220.3.51"},
    }

    dip := [][]string {
        {"82.220.3.51", "CH"},
        {"192.30.252.225", "US"},
        {"213.95.10.76", "DE"},
        {"94.76.229.204", "GB"},
        {"46.19.36.12", "NL"},
        {"46.30.209.1", "DK"},
        {"91.239.97.26", "SI"},
        {"14.1.44.230", "NZ"},
        {"52.76.214.87", "SG"},
        {"103.31.84.12", "MV"},
        {"212.63.210.241", "SE"},
        {"154.11.253.242", "CA"},
        {"128.139.197.81", "IL"},
        {"194.190.198.13", "RU"},
        {"84.88.14.229", "ES"},
        {"79.110.197.36", "PL"},
        {"175.45.73.66", "AU"},
        {"62.240.228.4", "FR"},
        {"200.238.130.54", "BR"},
        {"13.113.70.195", "JP"},
        {"37.252.235.214", "AT"},
        {"185.87.111.13", "FI"},
        {"52.66.51.117", "IN"},
        {"193.198.233.217", "HR"},
        {"118.67.200.190", "KH"},
        {"103.6.84.107", "HK"},
        {"78.128.211.50", "CZ"},
        {"87.238.39.42", "NO"},
        {"37.148.176.54", "BE"},
    }

    cfg := GeoIpConfig {
        Enable: true,
        Db: "../geoCity.mmdb",
    }
    logger.Default = logger.NewLogger(&logger.LogConfig{})

    g := NewGeoIp(&cfg)

    for i,_ := range sip {
        dest := new(IP_RRSet)
        for i,_ := range dip {
            _, _, cc, _ := g.GetGeoLocation(net.ParseIP(dip[i][0]))
            if cc != dip[i][1] {
                t.Fail()
            }
            r := IP_RR {
                Ip:  net.ParseIP(dip[i][0]),
            }
            dest.Data = append(dest.Data, r)
        }
        dest.Ttl = 100
        ips := g.GetMinimumDistance(net.ParseIP(sip[i][0]), dest.Data, map[string]interface{}{})
        log.Println("[DEBUG]", sip[i][0], " ", ips[0].Ip.String(), " ", len(ips))
        if sip[i][2] != ips[0].Ip.String() {
            t.Fail()
        }
    }
}

func TestGeoIpManual(t *testing.T) {
    sip := [][]string{
        {"212.83.32.45", "DE", "1.2.3.4"},
        {"80.67.163.250", "FR", "2.3.4.5"},
        {"154.11.253.242", "", "3.4.5.6"},
        {"127.0.0.1", "", "3.4.5.6"},
    }

    cfg := GeoIpConfig {
        Enable: true,
        Db: "../geoCity.mmdb",
    }
    logger.Default = logger.NewLogger(&logger.LogConfig{})

    g := NewGeoIp(&cfg)


    for i, _ := range sip {
        var dest IP_RRSet
        dest.Data = []IP_RR {
            { Ip: net.ParseIP("1.2.3.4"), Country: "DE"},
            { Ip: net.ParseIP("2.3.4.5"), Country: "FR"},
            { Ip: net.ParseIP("3.4.5.6"), Country: ""},
        }
        ips := g.GetSameCountry(net.ParseIP(sip[i][0]), dest.Data, map[string]interface{}{})
        if len(ips) != 1 {
            t.Fail()
        }
        log.Println("[DEBUG]", sip[i][1], sip[i][2], ips[0].Country, ips[0].Ip.String())
        if ips[0].Country != sip[i][1] || ips[0].Ip.String() != sip[i][2] {
            t.Fail()
        }
    }

}