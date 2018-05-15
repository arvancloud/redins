package server

import (
    "strconv"

    "github.com/miekg/dns"
    "arvancloud/redins/config"
)

func NewServer(config *config.RedinsConfig) *dns.Server {
        return &dns.Server {
            Addr: config.Server.Ip + ":" + strconv.Itoa(config.Server.Port),
            Net:  config.Server.Protocol,
        }
}