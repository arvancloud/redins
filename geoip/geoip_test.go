package geoip

import (
    "testing"
    "net"
    "github.com/hawell/redins/handler"
    "fmt"
)

func TestGeoIp(t *testing.T) {
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

    config := GeoipConfig {
        enable: true,
        dbName: "../geoCity.mmdb",
    }

    g := NewGeoIp(&config)

    for i,_ := range sip {
        dest := new(handler.Record)
        for i,_ := range dip {
            _, _, cc, _ := g.GetGeoLocation(net.ParseIP(dip[i][0]))
            if cc != dip[i][1] {
                t.Fail()
            }
            r := handler.A_Record {
                Ip:  net.ParseIP(dip[i][0]),
                Ttl: 100,
            }
            dest.A = append(dest.A, r)
        }
        g.GetMinimumDistance(net.ParseIP(sip[i][0]), dest)
        fmt.Println(sip[i][0], " ", dest.A[0].Ip.String(), " ", len(dest.A))
        if sip[i][2] != dest.A[0].Ip.String() {
            t.Fail()
        }
    }
}