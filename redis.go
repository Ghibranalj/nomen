package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// FetchDNS retrieves DNS records from Redis cache, falling back to DoH on miss
func FetchDNS(redisClient *redis.Client, name, dnsType string) ([]string, error) {
	ctx := context.Background()

	// Normalize: lowercase and strip trailing dot
	name = strings.ToLower(strings.TrimSuffix(name, "."))
	key := buildRedisKey(dnsType, name)

	// Try Redis first
	value, err := redisClient.Get(ctx, key).Result()
	if err == nil {
		// Parse response - try JSON array first, fallback to single value
		var answers []string
		if err := json.Unmarshal([]byte(value), &answers); err != nil {
			return []string{value}, nil
		}
		log.Printf("Redis hit: %s -> %v\n", key, answers)
		return answers, nil
	}

	// Cache miss - fetch from DoH
	log.Printf("Redis miss for %s, fetching from DoH\n", name)
	answers, err := QueryDOH(name, dnsType)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if len(answers) > 0 {
		CacheDNS(redisClient, name, dnsType, answers, 5*time.Minute)
	}

	return answers, nil
}

// CacheDNS stores DNS records in Redis with TTL
func CacheDNS(redisClient *redis.Client, name, dnsType string, records []string, ttl time.Duration) {
	ctx := context.Background()

	// Normalize: lowercase and strip trailing dot
	name = strings.ToLower(strings.TrimSuffix(name, "."))
	key := buildRedisKey(dnsType, name)

	// Marshal to JSON to handle multiple records
	value, err := json.Marshal(records)
	if err != nil {
		log.Printf("Failed to marshal records: %v\n", err)
		return
	}

	err = redisClient.Set(ctx, key, value, ttl).Err()
	if err != nil {
		log.Printf("Failed to cache in Redis: %v\n", err)
	} else {
		log.Printf("Cached: %s -> %v (TTL: %v)\n", key, records, ttl)
	}
}

// buildRedisKey creates a consistent Redis key format
func buildRedisKey(dnsType, name string) string {
	return fmt.Sprintf("%s:%s", dnsType, name)
}
