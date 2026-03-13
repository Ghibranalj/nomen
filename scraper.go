package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/go-routeros/routeros"
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

			log.Printf("  Lease: %s -> %s (%s) [%s]\n", domain, ipAddress, macAddress, status)

			// Store in Redis with TTL (key format: dnsType:domain)
			key := fmt.Sprintf("A:%s", domain)
			err := s.RedisClient.Set(s.ctx, key, ipAddress, s.TTL).Err()
			if err != nil {
				log.Printf("  Failed to store in Redis: %v\n", err)
			} else {
				log.Printf("  Stored: %s -> %s (TTL: %v)\n", key, ipAddress, s.TTL)
			}
		}
	}
}
