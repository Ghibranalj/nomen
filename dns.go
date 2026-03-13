package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

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

func (d *DNS) queryRedis(name, dnsType string) ([]string, error) {
	ctx := context.Background()
	key := fmt.Sprintf("%s:%s", dnsType, name)
	value, err := d.RedisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	// Try to parse as JSON array (for multiple answers)
	var answers []string
	if err := json.Unmarshal([]byte(value), &answers); err != nil {
		// If not JSON, treat as single value
		return []string{value}, nil
	}
	return answers, nil
}

func (d *DNS) cacheDOH(name, dnsType string, answers []string) {
	ctx := context.Background()
	key := fmt.Sprintf("%s:%s", dnsType, name)

	// Store as JSON array (for multiple answers)
	value, err := json.Marshal(answers)
	if err != nil {
		log.Printf("Failed to marshal answers: %v\n", err)
		return
	}

	// Cache DOH responses for 5 minutes
	ttl := 5 * time.Minute
	err = d.RedisClient.Set(ctx, key, value, ttl).Err()
	if err != nil {
		log.Printf("Failed to cache DOH response: %v\n", err)
	} else {
		log.Printf("Cached DOH response: %s -> %v (TTL: %v)\n", key, answers, ttl)
	}
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

	// Strip trailing dot from domain name for Redis lookup
	domainName := strings.TrimSuffix(question.Name, ".")
	log.Printf("Question: %s %s\n", domainName, dnsType)
	domainName = strings.ToLower(domainName)

	var answers []string
	var err error

	// Check Redis first
	answers, err = d.queryRedis(domainName, dnsType)
	if err != nil {
		log.Printf("Redis miss for %s\n", domainName)
		// Fetch from DOH if Redis miss (use original name with dot for DOH)
		answers, err = QueryDOH(question.Name, dnsType)
		if err != nil {
			log.Printf("DOH query failed: %v\n", err)
			w.WriteMsg(m)
			return
		}
		log.Printf("DOH answers: %v\n", answers)
		// Cache the DOH response
		if len(answers) > 0 {
			d.cacheDOH(domainName, dnsType, answers)
		}
	} else {
		log.Printf("Redis hit: %s -> %v\n", domainName, answers)
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
