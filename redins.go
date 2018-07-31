package main

import (
    "log"
    "sync"
    "os"
    "time"
    "io/ioutil"
    "encoding/json"

    "github.com/miekg/dns"
    "github.com/coredns/coredns/request"
    "arvancloud/redins/handler"
    "arvancloud/redins/server"
    "arvancloud/redins/eventlog"
    "arvancloud/redins/redis"
    "arvancloud/redins/upstream"
    "arvancloud/redins/geoip"
    "arvancloud/redins/healthcheck"
)

var (
    s []dns.Server
    h *handler.DnsRequestHandler
)

func handleRequest(w dns.ResponseWriter, r *dns.Msg) {
    // log.Printf("[DEBUG] handle request")
    state := request.Request{W: w, Req: r}

    h.HandleRequest(&state)
}

type RedinsConfig struct {
    Server []server.ServerConfig `json:"server,omitempty"`
    ErrorLog eventlog.LogConfig `json:"error_log,omitempty"`
    Handler handler.HandlerConfig `json:"handler,omitempty"`
}

func LoadConfig(path string) *RedinsConfig {
    config := &RedinsConfig {
        Server: []server.ServerConfig {
            {
                Ip:       "127.0.0.1",
                Port:     1053,
                Protocol: "udp",
            },
        },
        Handler: handler.HandlerConfig {
            Upstream: []upstream.UpstreamConfig {
                {
                    Ip:       "1.1.1.1",
                    Port:     53,
                    Protocol: "udp",
                    Timeout:  400,
                },
            },
            GeoIp: geoip.GeoIpConfig {
                Enable: false,
                Db: "geoCity.mmdb",
            },
            HealthCheck: healthcheck.HealthcheckConfig {
                Enable: false,
                MaxRequests: 10,
                UpdateInterval: 600,
                CheckInterval: 600,
                RedisStatusServer: redis.RedisConfig {
                    Ip: "127.0.0.1",
                    Port: 6379,
                    Password: "",
                    Prefix: "redins_",
                    Suffix: "_redins",
                    ConnectTimeout: 0,
                    ReadTimeout: 0,
                },
                Log: eventlog.LogConfig {
                    Enable: true,
                    Target: "file",
                    Level: "info",
                    Path: "/tmp/healthcheck.log",
                    Format: "json",
                    Sentry: eventlog.SentryConfig {
                        Enable: false,
                    },
                    Syslog: eventlog.SyslogConfig {
                        Enable: false,
                    },
                },
            },
            MaxTtl: 3600,
            CacheTimeout: 60,
            ZoneReload: 600,
            LogSourceLocation: false,
            UpstreamFallback: false,
            Redis: redis.RedisConfig {
                Ip: "127.0.0.1",
                Port: 6379,
                Password: "",
                Prefix: "redins_",
                Suffix: "_redins",
                ConnectTimeout: 0,
                ReadTimeout: 0,
            },
            Log: eventlog.LogConfig {
                Enable: true,
                Target: "file",
                Level: "info",
                Path: "/tmp/redins.log",
                Format: "json",
                Sentry: eventlog.SentryConfig {
                    Enable: false,
                },
                Syslog: eventlog.SyslogConfig {
                    Enable: false,
                },
            },
        },
        ErrorLog: eventlog.LogConfig {
            Enable: true,
            Target: "stdout",
            Level: "info",
            Format: "text",
            Sentry: eventlog.SentryConfig {
                Enable: false,
            },
            Syslog: eventlog.SyslogConfig {
                Enable: false,
            },
        },
    }
    raw, err := ioutil.ReadFile(path)
    if err != nil {
        log.Printf("[ERROR] cannot load file %s : %s", path, err)
        log.Printf("[INFO] loading default config")
        return config
    }
    err = json.Unmarshal(raw, config)
    if err != nil {
        log.Printf("[ERROR] cannot load json file")
        log.Printf("[INFO] loading default config")
        return config
    }
    return config
}

func main() {
    configFile := "config.json"
    if len(os.Args) > 1 {
        configFile = os.Args[1]
    }
    cfg := LoadConfig(configFile)

    eventlog.Logger = eventlog.NewLogger(&cfg.ErrorLog)

    s = server.NewServer(cfg.Server)

    h = handler.NewHandler(&cfg.Handler)

    dns.HandleFunc(".", handleRequest)

    var wg sync.WaitGroup
    for i := range s {
        go s[i].ListenAndServe()
        wg.Add(1)
        time.Sleep(1 * time.Second)
    }
    wg.Wait()
}
