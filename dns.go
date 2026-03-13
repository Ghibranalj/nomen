package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/miekg/dns"
	"github.com/redis/go-redis/v9"
)

type DNS struct {
	RedisClient *redis.Client
	Proto       string
	Port        int
	Bind        string
	server      *dns.Server
}

func (d *DNS) handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)

	if len(r.Question) == 0 {
		w.WriteMsg(m)
		return
	}

	question := r.Question[0]
	dnsType := dns.TypeToString[question.Qtype]
	domainName := question.Name

	log.Printf("Question: %s %s\n", strings.TrimSuffix(domainName, "."), dnsType)

	// Use consolidated FetchDNS (handles cache + DoH fallback)
	answers, err := FetchDNS(d.RedisClient, domainName, dnsType)
	if err != nil {
		log.Printf("DNS query failed: %v\n", err)
		w.WriteMsg(m)
		return
	}

	// Add answers to response
	for _, ans := range answers {
		rr, err := dns.NewRR(fmt.Sprintf("%s %s %s", question.Name, dnsType, ans))
		if err != nil {
			log.Printf("Failed to create RR: %v\n", err)
			continue
		}
		m.Answer = append(m.Answer, rr)
	}

	log.Printf("Response: %d answers\n", len(m.Answer))
	w.WriteMsg(m)
}

func NewDNS(proto string, port int, redisClient *redis.Client) *DNS {
	return &DNS{
		Proto:       proto,
		Port:        port,
		RedisClient: redisClient,
	}
}

func (d *DNS) Start() error {
	d.server = &dns.Server{
		Addr:    fmt.Sprintf(":%d", d.Port),
		Net:     d.Proto,
		Handler: dns.HandlerFunc(d.handleDNSRequest),
	}

	log.Printf("Starting DNS server on %s %s\n", d.Proto, d.server.Addr)
	return d.server.ListenAndServe()
}

func (d *DNS) Stop() error {
	if d.server != nil {
		return d.server.Shutdown()
	}
	return nil
}
