# table of contents

- [Configuration](#configuration)
    - [server](#server)
    - [handler](#handler)
    - [healthcheck](#healthcheck)
    - [geoip](#geoip)
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

~~~ini
[server]
ip = 127.0.0.1
prot = 1053
protocol = udp
~~~

* ip : ip address to bind
* port : port number to bind
* protocol : protocol; can be tcp or udp

### handler
dns query handler configuration

~~~ini
[handler]
ttl = 300
redis = redis_config
~~~

* ttl : default ttl in seconds
* redis : name of config file section containing redis configuration to use for handler

### healthcheck
healthcheck configuration

~~~ini
[healthcheck]
enable = true
max_requests = 10
update_interval = 10m
check_interval = 10m
redis_config = healthcheck_redis_config
redis_status = healthcheck_redis_status
log = healthcheck_log_config
~~~

* enable : enable/disable healthcheck
* max_requests : maximum number of simultanous healthcheck requests
* update_interval : time between checking for updated data from redis
* check_interval : time between two healthcheck requests
* redis_config : name of config file section containing redis configuration to use for healthcheck
* redis_status : name of config file section to use for healthcheck status
* log : name of config file section containing log configuration to use for healthcheck logs

### geoip
geoip configuration

~~~ini
[geoip]
enable = true
db = geoCity.mmdb
log = geoip_log_config
~~~

* enable : enable/disable geoip calculations
* db : maxminddb file to use
* log : name of config file section containing log configuration to use for geoip logs

### redis
redis configurations

~~~ini
ip = 127.0.0.1
port = 6379
password = 
prefix = test_
suffix = _test
connect_timeout = 200ms
read_timeout = 200ms
~~~

* ip : redis server ip
* port : redis server port
* password : redis password
* prefix : limit redis keys to those prefixed with this string
* suffix : limit redis keys to those suffixed with this string
* connect_timeout : time to wait for connecting to redis server
* read_timeout : time to wait for redis query results

### log
log configuration

~~~ini
enable = true
path = /tmp/log.log
~~~

* enable : enable/disable this log resource
* path : log output file path

### example
sample config:

~~~ini
[server]
ip = 127.0.0.1
port = 1053
protocol = udp

[handler]
ttl = 300
redis = handler_redis

[handler_redis]
ip = 127.0.0.1
port = 6379
password = 
prefix = test_
suffix = _test
connect_timeout = 200ms
read_timeout = 200ms

[geoip]
enable = true
mode = manual
db = geoCity.mmdb
log = geoip_log

[log]
enable = true
path = /tmp/dns.log

[healthcheck]
enable = true
max_requests = 10
update_interval = 10m
check_interval = 10m
redis = healthcheck_redis
log = healthcheck_log

[geoip_log]
enable = true
path = /tmp/geoip.log

[healthcheck_log]
enable = true
path = /tmp/healthcheck.log

[healthcheck_redis]
ip = 127.0.0.1
port = 6379
password =
prefix = healthcheck_
suffix = _healthcheck
connect_timeout = 200ms
read_timeout = 200ms

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
    "a":{
        "ip" : "1.2.3.4",
        "ttl" : 360,
        "country" : "US",
        "weight" : 10
    }
}
~~~

#### AAAA

~~~json
{
    "aaaa":{
        "ip" : "::1",
        "ttl" : 360,
        "country" : "US",
        "weight" : 10
    }
}
~~~

#### ANAME

~~~json
{
    "aname":{
        "location": "x.example.com.",
        "proxy": "1.1.1.1:53"
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
    "txt":{
        "text" : "this is a text",
        "ttl" : 360
    }
}
~~~

#### NS

~~~json
{
    "ns":{
        "host" : "ns1.example.com.",
        "ttl" : 360
    }
}
~~~

#### MX

~~~json
{
    "mx":{
        "host" : "mx1.example.com.",
        "priority" : 10,
        "ttl" : 360
    }
}
~~~

#### SRV

~~~json
{
    "srv":{
        "host" : "sip.example.com.",
        "port" : 555,
        "priority" : 10,
        "weight" : 100,
        "ttl" : 360
    }
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

