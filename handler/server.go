package handler

import (
    "strconv"

    "github.com/miekg/dns"
)


type ServerConfig struct {
    Ip       string `json:"ip,omitempty"`
    Port     int `json:"port,omitempty"`
    Protocol string `json:"protocol,omitempty"`
}

func NewServer(config []ServerConfig) []dns.Server {
    var servers []dns.Server
    for _, cfg := range config {
        server := dns.Server {
            Addr: cfg.Ip + ":" + strconv.Itoa(cfg.Port),
            Net:  cfg.Protocol,
        }
        servers = append(servers, server)
    }
    return servers
}