package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/redis/go-redis/v9"
)

// FetchDNS retrieves DNS records from Redis cache, falling back to DoH on miss
func FetchDNS(redisClient *redis.Client, name string, qtype uint16) (*dns.Msg, error) {
	ctx := context.Background()

	// Normalize: lowercase and strip trailing dot
	normalized := strings.ToLower(strings.TrimSuffix(name, "."))
	dnsTypeStr := dns.TypeToString[qtype]
	key := buildRedisKey(dnsTypeStr, normalized)

	// Try Redis first (local records from scraper/router)
	value, err := redisClient.Get(ctx, key).Result()
	if err == nil {
		msg := new(dns.Msg)
		if err := msg.Unpack([]byte(value)); err == nil {
			log.Printf("Redis hit: %s -> %d records\n", key, len(msg.Answer))
			return msg, nil
		}
	}

	// Cache miss - fetch from DoH
	log.Printf("Redis miss for %s, fetching from DoH\n", name)
	return QueryDOH(name, qtype)
}

// CacheDNS stores DNS messages in Redis using wire format.
// A cacheTTL of 0 means the key will be stored permanently without expiration.
func CacheDNS(redisClient *redis.Client, name, dnsType string, msg *dns.Msg, cacheTTL time.Duration) error {
	ctx := context.Background()
	name = strings.ToLower(strings.TrimSuffix(name, "."))
	key := buildRedisKey(dnsType, name)

	wireData, err := msg.Pack()
	if err != nil {
		return fmt.Errorf("pack DNS message: %w", err)
	}

	err = redisClient.Set(ctx, key, wireData, cacheTTL).Err()
	if err != nil {
		return fmt.Errorf("redis set: %w", err)
	}

	// When cacheTTL is 0, ensure the key has no expiration (permanent storage)
	if cacheTTL == 0 {
		if err := redisClient.Persist(ctx, key).Err(); err != nil {
			return fmt.Errorf("redis persist: %w", err)
		}
	}

	log.Printf("  Cached: %s -> %d records (cache TTL: %v)\n", key, len(msg.Answer), cacheTTL)
	return nil
}

// buildRedisKey creates a consistent Redis key format
func buildRedisKey(dnsType, name string) string {
	return fmt.Sprintf("%s:%s", dnsType, name)
}
