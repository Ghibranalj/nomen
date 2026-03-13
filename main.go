package main

import (
	"context"
	"fmt"
	"log"
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
	initRouterRecords(redisClient, cfg.Mikrotik, cfg.RouterTLD, cfg.DnsTTLMinutes)

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
func initRouterRecords(redisClient *redis.Client, mikrotiks []Mikrotik, routerTLD string, ttlMinutes int) {
	ttl := time.Duration(ttlMinutes) * time.Minute

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

		records := []DNSRecord{{
			Type: dns.TypeA,
			TTL:  uint32(ttl.Seconds()),
			Data: mikrotik.IP,
		}}
		CacheDNS(redisClient, routerDomain, "A", records, ttl)
	}
}
