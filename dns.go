package main

import (
	"fmt"
	"log"
	"net"
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

	// Add answers to response based on record type
	for _, record := range answers {
		var rr dns.RR
		var err error

		switch record.Type {
		case dns.TypeA:
			rr = &dns.A{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    record.TTL,
				},
				A: net.ParseIP(record.Data),
			}
		case dns.TypeAAAA:
			rr = &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    record.TTL,
				},
				AAAA: net.ParseIP(record.Data),
			}
		case dns.TypeCNAME:
			rr = &dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
					Ttl:    record.TTL,
				},
				Target: record.Data,
			}
		case dns.TypeMX:
			// MX records are stored as "priority target"
			priority := uint16(10) // default priority
			target := record.Data
			// Try to parse "priority target" format
			var p uint32
			n, _ := fmt.Sscanf(record.Data, "%d %s", &p, &target)
			if n == 2 {
				priority = uint16(p)
			}
			rr = &dns.MX{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeMX,
					Class:  dns.ClassINET,
					Ttl:    record.TTL,
				},
				Preference: priority,
				Mx:         target,
			}
		case dns.TypeNS:
			rr = &dns.NS{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeNS,
					Class:  dns.ClassINET,
					Ttl:    record.TTL,
				},
				Ns: record.Data,
			}
		case dns.TypeTXT:
			rr = &dns.TXT{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeTXT,
					Class:  dns.ClassINET,
					Ttl:    record.TTL,
				},
				Txt: []string{record.Data},
			}
		case dns.TypeSOA:
			// SOA records are stored as "ns mbox serial refresh retry expire minimum"
			// We'll try to use the generic parser for SOA
			fallthrough
		default:
			// For unknown types and SOA, try generic RR creation
			rr, err = dns.NewRR(fmt.Sprintf("%s %d IN %s %s",
				question.Name, record.TTL, dns.TypeToString[record.Type], record.Data))
			if err != nil {
				log.Printf("Failed to create RR for type %d: %v\n", record.Type, err)
				continue
			}
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
