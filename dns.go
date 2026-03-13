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
	domainName := question.Name

	log.Printf("Question: %s %s\n", strings.TrimSuffix(domainName, "."), dns.TypeToString[question.Qtype])

	// Use consolidated FetchDNS (handles cache + DoH fallback)
	// Now returns *dns.Msg directly, eliminating the need for conversion
	msg, err := FetchDNS(d.RedisClient, domainName, question.Qtype)
	if err != nil {
		log.Printf("DNS query failed: %v\n", err)
		m.SetRcode(r, 2) // SERVFAIL
		w.WriteMsg(m)
		return
	}

	if len(msg.Answer) == 0 {
		m.SetRcode(r, 3) // NXDOMAIN
		w.WriteMsg(m)
		return
	}

	// Directly use the response from FetchDNS (DoH wire format or cached local records)
	m.SetRcode(r, 0)
	m.Answer = msg.Answer
	m.Ns = msg.Ns
	m.Extra = msg.Extra

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
