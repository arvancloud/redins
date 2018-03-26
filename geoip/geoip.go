package geoip

import (
    "math"
    "net"
    "log"

    "github.com/oschwald/maxminddb-golang"
    "github.com/go-ini/ini"
    "github.com/hawell/redins/handler"
    "github.com/hawell/redins/eventlog"
)

type GeoIp struct {
    config *GeoipConfig
    db     *maxminddb.Reader
    logger *eventlog.EventLogger
}

type GeoipConfig struct {
    enable    bool
    dbName    string
    mode      string
    logConfig *eventlog.LoggerConfig
}

func LoadConfig(cfg *ini.File, section string) *GeoipConfig {
    geoipConfig := cfg.Section(section)
    logSection := geoipConfig.Key("log").MustString("log")
    return &GeoipConfig {
        enable:    geoipConfig.Key("enable").MustBool(true),
        dbName:    geoipConfig.Key("db").MustString("geoCity.mmdb"),
        mode:      geoipConfig.Key("mode").In("automatic", []string{"manual", "automatic"}),
        logConfig: eventlog.LoadConfig(cfg, logSection),
    }
}

func NewGeoIp(config *GeoipConfig) *GeoIp {
    g := &GeoIp {
        config: config,
    }
    var err error
    if config.enable {
        g.db, err = maxminddb.Open(config.dbName)
        if err != nil {
            log.Printf("[ERROR] cannot open maxminddb file %s", err)
            g.config.enable = false
        }
        g.logger = eventlog.NewLogger(g.config.logConfig)
    }
    // defer g.db.Close()
    return g
}

func (g *GeoIp) FilterGeoIp(sourceIp string, record *handler.Record) {
    if !g.config.enable {
        return
    }

    if g.config.mode == "automatic" {
        g.GetMinimumDistance(net.ParseIP(sourceIp), record)
    } else {
        g.GetSameCountry(net.ParseIP(sourceIp), record)
    }
}

func (g *GeoIp) GetSameCountry(sourceIp net.IP, record *handler.Record) {
    if g.config.enable == false {
        return
    }
    _, _, sourceCountry, err := g.GetGeoLocation(sourceIp)
    if err != nil {
        log.Printf("[ERROR] getSameCountry failed")
        return
    }

    if len(record.A) > 0 {
        matched := false
        matchedIndex := 0
        defaultIndex := 0
        for i := range record.A {
            if record.A[i].Country == sourceCountry {
                matched = true
                matchedIndex = i
                break
            }
            if record.A[i].Country == "" {
                defaultIndex = i
            }
        }
        if matched {
            record.A = []handler.A_Record{record.A[matchedIndex]}
        } else {
            record.A = []handler.A_Record{record.A[defaultIndex]}
        }
        g.logGeoIp(sourceIp, sourceCountry, record.A[0].Ip, record.A[0].Country)
    }

    if len(record.AAAA) > 0 {
        matched := false
        matchedIndex := 0
        defaultIndex := 0
        for i := range record.AAAA {
            if record.AAAA[i].Country == sourceCountry {
                matched = true
                matchedIndex = i
                break
            }
            if record.AAAA[i].Country == "" {
                defaultIndex = i
            }
        }
        if matched {
            record.AAAA = []handler.AAAA_Record{record.AAAA[matchedIndex]}
        } else {
            record.AAAA = []handler.AAAA_Record{record.AAAA[defaultIndex]}
        }
        g.logGeoIp(sourceIp, sourceCountry, record.AAAA[0].Ip, record.AAAA[0].Country)
    }
}

func (g *GeoIp) GetMinimumDistance(sourceIp net.IP, record *handler.Record) {
    if g.config.enable == false {
        return
    }
    minDistance := 1000.0
    index := -1
    slat, slong, _, err := g.GetGeoLocation(sourceIp)
    if err != nil {
        log.Printf("[ERROR] getMinimumDistance failed")
        return
    }
    for i := range record.A {
        destinationIp := record.A[i].Ip
        dlat, dlong, _, err := g.GetGeoLocation(destinationIp)
        d, err := g.getDistance(slat, slong, dlat, dlong)
        if err != nil {
            continue
        }
        if d < minDistance {
            minDistance = d
            index = i
        }
    }
    if index > -1 {
        record.A = []handler.A_Record {record.A[index] }
    }
    index = -1
    for i := range record.AAAA {
        destinationIp := record.AAAA[i].Ip
        dlat, dlong, _, err := g.GetGeoLocation(destinationIp)
        d, err := g.getDistance(slat, slong, dlat, dlong)
        if err != nil {
            continue
        }
        if d < minDistance {
            minDistance = d
            index = i
        }
    }
    if index > -1 {
        record.AAAA = []handler.AAAA_Record {record.AAAA[index] }
    }
}

func (g *GeoIp) getDistance(slat, slong, dlat, dlong float64) (float64, error) {
    deltaLat := (dlat - slat) * math.Pi / 180.0
    deltaLong := (dlong - slong) * math.Pi / 180.0
    slat = slat * math.Pi / 180.0
    dlat = dlat * math.Pi / 180.0

    a := math.Sin(deltaLat/2.0)*math.Sin(deltaLat/2.0) +
        math.Cos(slat)*math.Cos(dlat)* math.Sin(deltaLong/2.0)*math.Sin(deltaLong/2.0)
    c := 2.0 * math.Atan2(math.Sqrt(a), math.Sqrt(1.0-a))

    // log.Printf("[DEBUG] distance = ", c)

    return c, nil
}

func (g *GeoIp) GetGeoLocation(ip net.IP) (latitude float64, longitude float64, country string, err error) {
    if g.config.enable == false {
        return
    }
    var record struct {
        Location struct {
            Latitude        float64 `maxminddb:"latitude"`
            LongitudeOffset uintptr `maxminddb:"longitude"`
        } `maxminddb:"location"`
        Country struct {
            ISOCode string `maxminddb:"iso_code"`
        } `maxminddb:"country"`
    }
    // log.Printf("[DEBUG], ip : %s\n", ip)
    err = g.db.Lookup(ip, &record)
    if err != nil {
        log.Printf("[ERROR] lookup failed : %s", err)
        return 0, 0, "", err
    }
    g.db.Decode(record.Location.LongitudeOffset, &longitude)
    // log.Printf("[DEBUG] lat = ", record.Location.Latitude, " lang = ", longitude)
    return record.Location.Latitude, longitude, record.Country.ISOCode, nil
}

func (g *GeoIp) logGeoIp(sIp net.IP, sCountry string, dIp net.IP, dCountry string) {
    if g.config.logConfig.Enable == false {
        return
    }

    type geoIpLogData struct {
        SIp      string
        SCountry string
        DIp      string
        DCountry string
    }

    data := geoIpLogData {
        SIp: sIp.String(),
        SCountry: sCountry,
        DIp: dIp.String(),
        DCountry: dCountry,
    }

    g.logger.Log(data,"geoip")
}
