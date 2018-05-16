package server

import (
    "strconv"

    "github.com/miekg/dns"
    "arvancloud/redins/config"
)

func NewServer(config *config.RedinsConfig) []dns.Server {
    servers := []dns.Server{}
    for _, cfg := range config.Server {
        server := dns.Server {
            Addr: cfg.Ip + ":" + strconv.Itoa(cfg.Port),
            Net:  cfg.Protocol,
        }
        servers = append(servers, server)
    }
    return servers
}