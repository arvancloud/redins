package handler

import (
	"strconv"

	"github.com/miekg/dns"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
)

type TlsConfig struct {
	Enable   bool `json:"enable"`
	CertPath string `json:"cert_path"`
	KeyPath  string `json:"key_path"`
	CaPath   string `json:"ca_path"`
}

type ServerConfig struct {
	Ip       string `json:"ip,omitempty"`
	Port     int    `json:"port,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Tls      TlsConfig `json:"tls,omitempty"`
}

func loadRoots(caPath string) *x509.CertPool {
	if caPath == "" {
		return nil
	}

	roots := x509.NewCertPool()
	pem, err := ioutil.ReadFile(caPath)
	if err != nil {
		return nil
	}
	ok := roots.AppendCertsFromPEM(pem)
	if !ok {
		return nil
	}
	return roots
}

func loadTlsConfig(cfg TlsConfig) *tls.Config {
	root := loadRoots(cfg.CaPath)
	if cfg.KeyPath == "" || cfg.CertPath == "" {
		return &tls.Config{RootCAs: root}
	}
	cert, err := tls.LoadX509KeyPair(cfg.CertPath, cfg.KeyPath)
	if err != nil {
		return nil
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}, RootCAs: root}
}

func NewServer(config []ServerConfig) []dns.Server {
	var servers []dns.Server
	for _, cfg := range config {
		server := dns.Server{
			Addr: cfg.Ip + ":" + strconv.Itoa(cfg.Port),
			Net:  cfg.Protocol,
		}
		if cfg.Tls.Enable {
			server.TLSConfig = loadTlsConfig(cfg.Tls)
		}
		servers = append(servers, server)
	}
	return servers
}
