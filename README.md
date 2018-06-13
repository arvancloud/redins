# table of contents

- [Configuration](#configuration)
    - [server](#server)
    - [handler](#handler)
    - [healthcheck](#healthcheck)
    - [geoip](#geoip)
    - [upstream](#upstream)
    - [error log](#error_log)
    - [redis](#redis)
    - [log](#log)
    - [example](#example)
- [Zone format in redis](#zone-format-in-redis-db)
    - [zones](#zones)
    - [dns RRs](#dns-rrs)
        - [A](#a)
        - [AAAA](#aaaa)
        - [ANAME](#aname)
        - [CNAME](#cname)
        - [TXT](#txt)
        - [NS](#ns)
        - [MX](#mx)
        - [SRV](#srv)
        - [SOA](#soa)
        - [Config](#config)
    - [example](#zone-example)
    

*redins* enables reading zone data from redis database.

## Configuration

### server
dns listening server configuration

~~~json
"server": {
  "ip": "127.0.0.1",
  "port": 1053,
  "protocol": "udp"
}
~~~

* ip : ip address to bind, default: 127.0.0.1
* port : port number to bind, default: 1053
* protocol : protocol; can be tcp or udp, default: udp

### handler
dns query handler configuration

~~~json
"handler": {
    "default_ttl": 300,
    "cache_timeout": 60,
    "zone_reload": 600,
    "log_source_location": false,
    "upstream_fallback": false,
    "redis": {
      "ip": "127.0.0.1",
      "port": 6379,
      "password": "",
      "prefix": "test_",
      "suffix": "_test",
      "connect_timeout": 0,
      "read_timeout": 0
    },
    "log": {
    "enable": true,
    "level": "info",
    "target": "file",
    "format": "json",
    "path": "/tmp/redins.log"
    }
}
~~~

* default_ttl : default ttl in seconds, default: 300
* cache_timeout : time in seconds before cached responses expire
* zone_reload : time in seconds before zone data is reloaded from redis
* log_source_location : enable logging source location of every request
* upstream_fallback : enable using upstream for querying non-authoritative requests
* redis : redis configuration to use for handler
* log : log configuration to use for handler

### healthcheck
healthcheck configuration

~~~json
  "healthcheck": {
    "enable": true,
    "max_requests": 10,
    "update_interval": 600,
    "check_interval": 600,
    "redis": {
      "ip": "127.0.0.1",
      "port": 6379,
      "password": "",
      "prefix": "healthcheck_",
      "suffix": "_healthcheck",
      "connect_timeout": 0,
      "read_timeout": 0
    },
    "log": {
      "enable": true,
      "level": "info",
      "target": "file",
      "format": "json",
      "path": "/tmp/healthcheck.log"
    }
  }
~~~

* enable : enable/disable healthcheck, default: disable
* max_requests : maximum number of simultanous healthcheck requests, deafult: 10
* update_interval : time between checking for updated data from redis in seconds, default: 300
* check_interval : time between two healthcheck requests in seconds, default: 600
* redis : redis configuration to use for healthcheck stats
* log : log configuration to use for healthcheck logs

### geoip
geoip configuration

~~~json
  "geoip": {
    "enable": true,
    "db": "geoCity.mmdb"
  }
~~~

* enable : enable/disable geoip calculations, default: disable
* db : maxminddb file to use, default: geoCity.mmdb

### upstream

~~~json
"upstream": {
    "ip": "1.1.1.1",
    "port": 53,
    "protocol": "udp",
    "timeout": 400
},
~~~

* ip : upstream ip address, default: 1.1.1.1
* port : upstream port number, deafult: 53
* protocol : upstream protocol, default : udp
* timeout : request timeout in milliseconds, default: 400

### error_log
log configuration for error, debug, ... messages

~~~json
"log": {
  "enable": true,
  "level": "info",
  "target": "file",
  "format": "json",
  "path": "/tmp/redins.log"
}
~~~

### redis
redis configurations

~~~json
"redis": {
  "ip": "127.0.0.1",
  "port": 6379,
  "password": "",
  "prefix": "test_",
  "suffix": "_test",
  "connect_timeout": 0,
  "read_timeout": 0
},
~~~

* ip : redis server ip, default: 127.0.0.1
* port : redis server port, deafult: 6379
* password : redis password, deafult: ""
* prefix : limit redis keys to those prefixed with this string
* suffix : limit redis keys to those suffixed with this string
* connect_timeout : time to wait for connecting to redis server in milliseconds, deafult: 0 
* read_timeout : time to wait for redis query results in milliseconds, default: 0

### log
log configuration

~~~json
"log": {
  "enable": true,
  "level": "info",
  "target": "file",
  "format": "json",
  "path": "/tmp/redins.log"
}
~~~

* enable : enable/disable this log resource, default: disable
* level : log level, can be debug, info, warning, error, default: info
* target : log target, can be stdout, stderr, file, default: stdout
* format : log format, can be text, json, default: text
* path : log output file path, default: 

### example
sample config:

~~~json
{
  "server": {
      "ip": "127.0.0.1",
      "port": 1053,
      "protocol": "udp"
    },
  "handler": {
    "default_ttl": 300,
    "cache_timeout": 60,
    "zone_reload": 600,
    "log_source_location": false,
    "upstream_fallback": false,
    "redis": {
      "ip": "127.0.0.1",
      "port": 6379,
      "password": "",
      "prefix": "test_",
      "suffix": "_test",
      "connect_timeout": 0,
      "read_timeout": 0
    },
    "log": {
      "enable": true,
      "level": "info",
      "target": "file",
      "format": "json",
      "path": "/tmp/redins.log"
    }
  },
  "upstream": {
    "ip": "1.1.1.1",
    "port": 53,
    "protocol": "udp"
  },
  "geoip": {
    "enable": true,
    "db": "geoCity.mmdb"
  },
  "healthcheck": {
    "enable": true,
    "max_requests": 10,
    "update_interval": 600,
    "check_interval": 600,
    "redis": {
      "ip": "127.0.0.1",
      "port": 6379,
      "password": "",
      "prefix": "healthcheck_",
      "suffix": "_healthcheck",
      "connect_timeout": 0,
      "read_timeout": 0
    },
    "log": {
      "enable": true,
      "level": "info",
      "target": "file",
      "format": "json",
      "path": "/tmp/healthcheck.log"
    }
  },
  "error_log": {
      "enable": true,
      "level": "info",
      "target": "stdout",
      "format": "json",
      "path": "/tmp/healthcheck.log"
  }
}
~~~

## zone format in redis db

### zones

each zone is stored in redis as a hash map with *zone* as key

~~~
redis-cli>KEYS *
1) "example.com."
2) "example.net."
redis-cli>
~~~

### dns RRs 

dns RRs are stored in redis as json strings inside a hash map using address as field key.
*@* is used for zone's own RR values.

#### A

~~~json
{
    "a":[{
        "ip" : "1.2.3.4",
        "ttl" : 360,
        "country" : "US",
        "weight" : 10
    }]
}
~~~

#### AAAA

~~~json
{
    "aaaa":[{
        "ip" : "::1",
        "ttl" : 360,
        "country" : "US",
        "weight" : 10
    }]
}
~~~

#### ANAME

~~~json
{
    "aname":{
        "location": "x.example.com."
    }
}
~~~

#### CNAME

~~~json
{
    "cname":{
        "host" : "x.example.com.",
        "ttl" : 360
    }
}
~~~

#### TXT

~~~json
{
    "txt":[{
        "text" : "this is a text",
        "ttl" : 360
    }]
}
~~~

#### NS

~~~json
{
    "ns":[{
        "host" : "ns1.example.com.",
        "ttl" : 360
    }]
}
~~~

#### MX

~~~json
{
    "mx":[{
        "host" : "mx1.example.com.",
        "preference" : 10,
        "ttl" : 360
    }]
}
~~~

#### SRV

~~~json
{
    "srv":[{
        "target" : "sip.example.com.",
        "port" : 555,
        "priority" : 10,
        "weight" : 100,
        "ttl" : 360
    }]
}
~~~

#### SOA

~~~json
{
    "soa":{
        "ttl" : 100,
        "mbox" : "hostmaster.example.com.",
        "ns" : "ns1.example.com.",
        "refresh" : 44,
        "retry" : 55,
        "expire" : 66
    }
}
~~~

#### config

~~~json
{
    "config":{
        "ip_filter_mode": "multi",
        "health_check":{
            "enable":true,
            "uri": "/hc/test.html",
            "port": 8080,
            "protocol": "https",
            "up_count":3,
            "down_count":-3,
            "timeout":1000
        }
    }
}
~~~

`ip-filter_mode` : filtering mode:
* multi : return all A or AAAA records
* multi_rr : shuffle records for each request
* rr : weighted round robin selection
* geo_location : nearest geographical location
* geo_country : match with same country as source ip

`enable` enable/disable healthcheck for this host:ip

`uri` uri to use in healthcheck request

`port` port to use in healthcheck request

`protocol` protocol to use in healthcheck request, can be http or https

`up_count` number of successful healthcheck requests to consider an ip valid

`down_count` number of unsuccessful healthcheck requests to consider an ip invalid

`timeout` time to wait for a healthcheck response

### zone example

~~~
$ORIGIN example.net.
 example.net.                 300 IN  SOA   <SOA RDATA>
 example.net.                 300     NS    ns1.example.net.
 example.net.                 300     NS    ns2.example.net.
 *.example.net.               300     TXT   "this is a wildcard"
 *.example.net.               300     MX    10 host1.example.net.
 sub.*.example.net.           300     TXT   "this is not a wildcard"
 host1.example.net.           300     A     5.5.5.5
 _ssh.tcp.host1.example.net.  300     SRV   <SRV RDATA>
 _ssh.tcp.host2.example.net.  300     SRV   <SRV RDATA>
 subdel.example.net.          300     NS    ns1.subdel.example.net.
 subdel.example.net.          300     NS    ns2.subdel.example.net.
~~~

above zone data should be stored at redis as follow:

~~~
redis-cli> hgetall example.net.
 1) "_ssh._tcp.host1"
 2) "{\"srv\":[{\"ttl\":300, \"target\":\"tcp.example.com.\",\"port\":123,\"priority\":10,\"weight\":100}]}"
 3) "*"
 4) "{\"txt\":[{\"ttl\":300, \"text\":\"this is a wildcard\"}],\"mx\":[{\"ttl\":300, \"host\":\"host1.example.net.\",\"preference\": 10}]}"
 5) "host1"
 6) "{\"a\":[{\"ttl\":300, \"ip\":\"5.5.5.5\"}]}"
 7) "sub.*"
 8) "{\"txt\":[{\"ttl\":300, \"text\":\"this is not a wildcard\"}]}"
 9) "_ssh._tcp.host2"
10) "{\"srv\":[{\"ttl\":300, \"target\":\"tcp.example.com.\",\"port\":123,\"priority\":10,\"weight\":100}]}"
11) "subdel"
12) "{\"ns\":[{\"ttl\":300, \"host\":\"ns1.subdel.example.net.\"},{\"ttl\":300, \"host\":\"ns2.subdel.example.net.\"}]}"
13) "@"
14) "{\"soa\":{\"ttl\":300, \"minttl\":100, \"mbox\":\"hostmaster.example.net.\",\"ns\":\"ns1.example.net.\",\"refresh\":44,\"retry\":55,\"expire\":66},\"ns\":[{\"ttl\":300, \"host\":\"ns1.example.net.\"},{\"ttl\":300, \"host\":\"ns2.example.net.\"}]}"
redis-cli> 
~~~

