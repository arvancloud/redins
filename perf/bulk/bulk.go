package main

import (
	"bufio"
	"fmt"
	"github.com/miekg/dns"
	"os"
	"time"
)

func main() {
	client := &dns.Client{
		Net:     "udp",
		Timeout: time.Millisecond * 100,
	}

	fq, err := os.Open("../query.txt")
	if err != nil {
		fmt.Println("cannot open query.txt")
		return
	}
	defer fq.Close()
	rq := bufio.NewReader(fq)
	var duration time.Duration
	for {
		line, err := rq.ReadString('\n')
		if err != nil {
			break
		}
		var queryAddr, queryResult string
		// fmt.Println("line = ", line)
		fmt.Sscan(line, &queryAddr, &queryResult)
		// fmt.Println("addr = ", queryAddr, "result = ", queryResult)
		m := new(dns.Msg)
		m.SetQuestion(queryAddr, dns.TypeA)
		r, rtt, err := client.Exchange(m, "localhost:1053")
		if err != nil {
			fmt.Println("error: ", err)
			break
		}
		if r.Rcode != dns.RcodeSuccess {
			fmt.Println("bad response : ", r.Rcode)
			break
		}
		if len(r.Answer) == 0 {
			fmt.Println("empty response")
			break
		}
		a := r.Answer[0].(*dns.A)
		if a.A.String() != queryResult {
			fmt.Printf("error: incorrect answer : expected %s got %s", queryResult, a.A.String())
			break
		}
		duration += rtt
	}
	fmt.Println(duration)
}
