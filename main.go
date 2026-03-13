package main

import (
	"context"
	"log"

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

	// Test Redis connection
	_, err := redisClient.Ping(context.Background()).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Connected to Redis")

	scraper := NewScraper(redisClient, cfg.Mikrotik, cfg.ScrapeIntervalMinutes, cfg.DnsTTLMinutes)
	scraper.Start()

	dns := NewDNS(cfg.Proto, cfg.Port, redisClient)

	dns.Start()
}
