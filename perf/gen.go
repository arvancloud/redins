package main

import (
	"bufio"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/gomodule/redigo/redis"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyz"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)
const suffix = ".tst."

var src = rand.NewSource(time.Now().UnixNano())

func RandomString(n int) string {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

func main() {
	zonesPtr := flag.Int("zones", 10, "number of zones")
	entriesPtr := flag.Int("entries", 100, "number of entries per zone")
	// missChancePtr := flag.Int("miss", 30, "miss chance")

	redisAddrPtr := flag.String("addr", "localhost:6379", "redis address")
	// redisAuthPtr := flag.String("auth", "foobared", "redis authentication")

	flag.Parse()

	opts := []redis.DialOption{}
	// opts = append(opts, redis.DialPassword(*redisAuthPtr))
	con, err := redis.Dial("tcp", *redisAddrPtr, opts...)
	if err != nil {
		fmt.Println("redis connection error")
		return
	}

	con.Do("EVAL", "return redis.call('del', unpack(redis.call('keys', ARGV[1])))", 0, "*")

	fq, err := os.Create("query.txt")
	if err != nil {
		fmt.Println("cannot open file query.txt")
		return
	}
	defer fq.Close()
	wq := bufio.NewWriter(fq)

	for i := 0; i < *zonesPtr; i++ {
		zoneName := RandomString(15) + suffix
		fz, err := os.Create(zoneName)
		if err != nil {
			fmt.Println("cannot open file " + zoneName)
			return
		}
		defer fz.Close()
		con.Do("SADD", "redins:zones", zoneName)
		wz := bufio.NewWriter(fz)
		wz.WriteString("$ORIGIN " + zoneName + "\n" +
			"$TTL 86400\n\n" +
			"@       SOA ns1 hostmaster (\n" +
			"1      ; serial\n" +
			"7200   ; refresh\n" +
			"30M    ; retry\n" +
			"3D     ; expire\n" +
			"900    ; ncache\n" +
			")\n" +
			"@ NS ns1." + zoneName + "\n" +
			"ns1 A 1.2.3.4\n\n")

		for j := 0; j < *entriesPtr; j++ {
			location := RandomString(15)
			ip := fmt.Sprintf("%d.%d.%d.%d", rand.Intn(256), rand.Intn(256), rand.Intn(256), rand.Intn(256))

			con.Do("HSET", "redins:zones:"+zoneName, location, `{"a":{"ttl":300, "records":[{"ip":"`+ip+`"}]}}`)

			wq.WriteString(location + "." + zoneName + " " + ip + "\n")

			wz.WriteString(location + " A " + ip + "\n")
		}
		wz.Flush()
	}
	wq.Flush()
}
