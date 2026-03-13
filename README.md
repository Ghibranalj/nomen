# Nomen

Local DNS server that resolves hostnames from Mikrotik DHCP leases.

**Nomen** is Latin for "name" — fitting for a DNS server that gives names to your local devices.

## Features

- Scrapes DHCP leases from Mikrotik routers via RouterOS API
- Caches records in Redis with configurable TTL
- Falls back to DNS-over-HTTPS (Cloudflare) for external queries
- Supports custom TLDs for local domains
- Multiple Mikrotik routers supported

## Configuration

Edit `config.yaml`:

```yaml
Mikrotik:
  - IP: "10.0.64.1"
    Port: 8728
    User: "admin"
    Password: "password"
    TLD: "alsut"          # Leave empty for no TLD

DohServer: "https://1.1.1.1/dns-query"
RedisURL: "localhost:6379"
Port: 53
Proto: "udp"
ScrapeIntervalMinutes: 5
DnsTTLMinutes: 10
```

## Running

Start Redis:
```bash
docker compose up -d
```

Run the server:
```bash
go run .
```

## Testing

Query your DNS server:
```bash
dig @localhost -p 53 creeprair.alsut
```

## Requirements

- Go 1.25+
- Redis
- Mikrotik router with API access
