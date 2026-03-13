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
func FetchDNS(redisClient *redis.Client, name, dnsType string) ([]DNSRecord, error) {
	ctx := context.Background()

	// Normalize: lowercase and strip trailing dot
	key := buildRedisKey(dnsType, strings.ToLower(strings.TrimSuffix(name, ".")))

	// Try Redis first
	value, err := redisClient.Get(ctx, key).Result()
	if err == nil {
		var records []DNSRecord
		if err := json.Unmarshal([]byte(value), &records); err == nil {
			log.Printf("Redis hit: %s -> %d records\n", key, len(records))
			return records, nil
		}
	}

	// Cache miss - fetch from DoH
	log.Printf("Redis miss for %s, fetching from DoH\n", name)
	records, err := QueryDOH(name, dnsType)
	if err != nil {
		return nil, err
	}

	return records, nil
}

// CacheDNS stores DNS records in Redis with TTL
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
		log.Printf("Cached: %s -> %d records (cache TTL: %v)\n", key, len(records), cacheTTL)
	}
}

// buildRedisKey creates a consistent Redis key format
func buildRedisKey(dnsType, name string) string {
	return fmt.Sprintf("%s:%s", dnsType, name)
}
