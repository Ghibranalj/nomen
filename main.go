package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

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
