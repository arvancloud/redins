package server

import (
    "github.com/go-ini/ini"
    "github.com/miekg/dns"
    "strconv"
)
type ServerConfig struct {
    ip       string
    port     int
    protocol string
}

func LoadConfig(cfg *ini.File, section string) *ServerConfig {
    serverConfig := cfg.Section(section)
    return &ServerConfig {
        ip: serverConfig.Key("ip").MustString("127.0.0.1"),
        port: serverConfig.Key("port").MustInt(1053),
        protocol: serverConfig.Key("protocol").MustString("udp"),
    }
}

func NewServer(config *ServerConfig) *dns.Server {
    return &dns.Server {
        Addr: config.ip + ":" + strconv.Itoa(config.port),
        Net: config.protocol,
    }
}