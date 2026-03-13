package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/go-routeros/routeros"
	"github.com/miekg/dns"
	"github.com/redis/go-redis/v9"
)

type Scraper struct {
	RedisClient *redis.Client
	Mikrotiks   []Mikrotik
	Interval    time.Duration
	TTL         time.Duration
	ticker      *time.Ticker
	stop        chan struct{}
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewScraper(redisClient *redis.Client, mikrotiks []Mikrotik, intervalMinutes int, ttlMinutes int) *Scraper {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scraper{
		RedisClient: redisClient,
		Mikrotiks:   mikrotiks,
		Interval:    time.Duration(intervalMinutes) * time.Minute,
		TTL:         time.Duration(ttlMinutes) * time.Minute,
		stop:        make(chan struct{}),
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (s *Scraper) Start() {
	log.Printf("Starting scraper with interval: %v\n", s.Interval)

	go s.scrape()

	s.ticker = time.NewTicker(s.Interval)
	go func() {
		for {
			select {
			case <-s.ticker.C:
				go s.scrape()
			case <-s.stop:
				return
			case <-s.ctx.Done():
				return
			}
		}
	}()
}

func (s *Scraper) Stop() {
	log.Println("Stopping scraper")
	s.cancel()
	if s.ticker != nil {
		s.ticker.Stop()
	}
	close(s.stop)
}

func (s *Scraper) scrape() {
	log.Println("Scraping Mikrotik DHCP leases...")
	for _, mikrotik := range s.Mikrotiks {
		address := fmt.Sprintf("%s:%d", mikrotik.IP, mikrotik.Port)
		log.Printf("Scraping to %s\n", address)

		client, err := routeros.Dial(address, mikrotik.User, mikrotik.Password)
		if err != nil {
			log.Printf("Failed to connect to %s: %v\n", address, err)
			continue
		}
		defer client.Close()

		response, err := client.Run("/ip/dhcp-server/lease/print")
		if err != nil {
			log.Printf("Failed to get leases from %s: %v\n", address, err)
			continue
		}

		for _, re := range response.Re {
			hostname := re.Map["host-name"]
			ipAddress := re.Map["address"]
			macAddress := re.Map["mac-address"]
			status := re.Map["status"]

			if hostname == "" {
				continue
			}

			// Build domain name (handle empty TLD)
			var domain string
			if mikrotik.TLD != "" {
				domain = fmt.Sprintf("%s.%s", hostname, mikrotik.TLD)
			} else {
				domain = hostname
			}

			domain = strings.ToLower(domain)

			log.Printf("  Lease: %s -> %s (%s) [%s]\n", domain, ipAddress, macAddress, status)

			// Create DNS message for A record
			msg := new(dns.Msg)
			msg.SetQuestion(domain, dns.TypeA)
			msg.Response = true
			msg.RecursionAvailable = true

			rr := &dns.A{
				Hdr: dns.RR_Header{
					Name:   domain,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    uint32(s.TTL.Seconds()),
				},
				A: net.ParseIP(ipAddress),
			}
			msg.Answer = append(msg.Answer, rr)

			// Cache using wire format
			CacheDNS(s.RedisClient, domain, "A", msg, s.TTL)
		}
	}
}
