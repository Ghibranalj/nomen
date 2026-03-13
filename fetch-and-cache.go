package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/redis/go-redis/v9"
)

// FetchDNS retrieves DNS records from Redis cache, falling back to DoH on miss
// Returns *dns.Msg to support both cached records and DoH wire format responses
func FetchDNS(redisClient *redis.Client, name string, qtype uint16) (*dns.Msg, error) {
	ctx := context.Background()

	// Normalize: lowercase and strip trailing dot
	normalized := strings.ToLower(strings.TrimSuffix(name, "."))
	dnsTypeStr := dns.TypeToString[qtype]
	key := buildRedisKey(dnsTypeStr, normalized)

	// Try Redis first (for local records like router, DHCP)
	value, err := redisClient.Get(ctx, key).Result()
	if err == nil {
		var records []DNSRecord
		if err := json.Unmarshal([]byte(value), &records); err == nil {
			log.Printf("Redis hit: %s -> %d records\n", key, len(records))
			return dnsRecordsToMsg(name, qtype, records)
		}
	}

	// Cache miss - fetch from DoH (DoH responses are NOT cached per user request)
	log.Printf("Redis miss for %s, fetching from DoH\n", name)
	return QueryDOH(name, qtype)
}

// dnsRecordsToMsg converts []DNSRecord to *dns.Msg (used for cached local records)
func dnsRecordsToMsg(name string, qtype uint16, records []DNSRecord) (*dns.Msg, error) {
	m := new(dns.Msg)
	m.SetQuestion(name, qtype)
	m.Response = true
	m.RecursionAvailable = true

	for _, record := range records {
		var rr dns.RR
		var err error

		switch record.Type {
		case dns.TypeA:
			rr = &dns.A{
				Hdr: dns.RR_Header{
					Name:   name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    record.TTL,
				},
				A: net.ParseIP(record.Data),
			}
		case dns.TypeAAAA:
			rr = &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   name,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    record.TTL,
				},
				AAAA: net.ParseIP(record.Data),
			}
		case dns.TypeCNAME:
			rr = &dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   name,
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
					Ttl:    record.TTL,
				},
				Target: record.Data,
			}
		case dns.TypeMX:
			priority := uint16(10)
			target := record.Data
			var p uint32
			n, _ := fmt.Sscanf(record.Data, "%d %s", &p, &target)
			if n == 2 {
				priority = uint16(p)
			}
			rr = &dns.MX{
				Hdr: dns.RR_Header{
					Name:   name,
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
					Name:   name,
					Rrtype: dns.TypeNS,
					Class:  dns.ClassINET,
					Ttl:    record.TTL,
				},
				Ns: record.Data,
			}
		case dns.TypeTXT:
			rr = &dns.TXT{
				Hdr: dns.RR_Header{
					Name:   name,
					Rrtype: dns.TypeTXT,
					Class:  dns.ClassINET,
					Ttl:    record.TTL,
				},
				Txt: []string{record.Data},
			}
		case dns.TypeSOA:
			fallthrough
		default:
			rr, err = dns.NewRR(fmt.Sprintf("%s %d IN %s %s",
				name, record.TTL, dns.TypeToString[record.Type], record.Data))
			if err != nil {
				log.Printf("Failed to create RR for type %d: %v\n", record.Type, err)
				continue
			}
		}

		m.Answer = append(m.Answer, rr)
	}

	return m, nil
}

// CacheDNS stores DNS records in Redis with TTL (used for router, DHCP records)
func CacheDNS(redisClient *redis.Client, name, dnsType string, records []DNSRecord, cacheTTL time.Duration) {
	ctx := context.Background()

	// Normalize: lowercase and strip trailing dot
	name = strings.ToLower(strings.TrimSuffix(name, "."))
	key := buildRedisKey(dnsType, name)

	value, err := json.Marshal(records)
	if err != nil {
		log.Printf("Failed to marshal records: %v\n", err)
		return
	}

	err = redisClient.Set(ctx, key, value, cacheTTL).Err()
	if err != nil {
		log.Printf("Failed to cache in Redis: %v\n", err)
	} else {
		log.Printf("  Cached: %s -> %d records (cache TTL: %v)\n", key, len(records), cacheTTL)
	}
}

// buildRedisKey creates a consistent Redis key format
func buildRedisKey(dnsType, name string) string {
	return fmt.Sprintf("%s:%s", dnsType, name)
}
