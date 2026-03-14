package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/miekg/dns"
	"github.com/redis/go-redis/v9"
)

func main() {
	if err := ReadConfig("config.yaml"); err != nil {
		panic(err)
	}

	// Create Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: cfg.RedisURL,
	})
	defer redisClient.Close()

	// Test Redis connection
	_, err := redisClient.Ping(context.Background()).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Connected to Redis")

	// Initialize static router records
	if err := initRouterRecords(redisClient, cfg.Mikrotik, cfg.RouterTLD, cfg.DnsTTLMinutes); err != nil {
		log.Fatalf("Failed to initialize router records: %v", err)
	}

	scraper := NewScraper(redisClient, cfg.Mikrotik, cfg.ScrapeIntervalMinutes, cfg.DnsTTLMinutes)
	scraper.Start()

	dns := NewDNS(cfg.Proto, cfg.Port, redisClient)

	go func() {
		err := dns.Start()
		if err != nil {
			log.Fatalf("DNS server failed %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")

	// Stop components
	scraper.Stop()
	dns.Stop()

	log.Println("Shutdown complete")
}

// initRouterRecords stores static router A records in Redis at startup
func initRouterRecords(redisClient *redis.Client, mikrotiks []Mikrotik, routerTLD string, ttlMinutes int) error {

	log.Println("Initializing router DNS records...")

	for _, mikrotik := range mikrotiks {
		if mikrotik.Name == "" {
			log.Printf("  Skipping router at %s (no name configured)\n", mikrotik.IP)
			continue
		}

		var routerDomain string
		if routerTLD != "" {
			routerDomain = fmt.Sprintf("%s.%s", mikrotik.Name, routerTLD)
		} else {
			routerDomain = mikrotik.Name
		}
		routerDomain = strings.ToLower(routerDomain)

		msg := new(dns.Msg)
		msg.SetQuestion(routerDomain+".", dns.TypeA)
		msg.Response = true
		msg.RecursionAvailable = true

		dnsTTL := cfg.DnsTTLMinutes
		rr := &dns.A{
			Hdr: dns.RR_Header{
				Name:   routerDomain + ".",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    uint32(dnsTTL),
			},
			A: net.ParseIP(mikrotik.IP),
		}
		msg.Answer = append(msg.Answer, rr)

		ttl := time.Duration(0) // ZERO = permanent storage

		if err := CacheDNS(redisClient, routerDomain, "A", msg, ttl); err != nil {
			return fmt.Errorf("cache router record %s: %w", routerDomain, err)
		}
	}
	return nil
}
