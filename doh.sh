#!/usr/bin/env bash

# DNS-over-HTTPS test using Cloudflare (1.1.1.1)

# Query for A record of example.com
curl -s "https://1.1.1.1/dns-query?name=example.com&type=A" \
  -H "accept: application/dns-json"

echo -e "\n---\n"

# Query for A record with dig-style output (using +short option)
curl -s "https://1.1.1.1/dns-query?name=example.com&type=A" \
  -H "accept: application/dns-json" | jq -r '.Answer[].data'

echo -e "\n---\n"

# Query with POST (binary DNS wireformat)
printf "\x01\x00\x00\x01\x00\x00\x00\x00\x00\x01\x07\x65\x78\x61\x6d\x70\x6c\x65\x03\x63\x6f\x6d\x00\x00\x01\x00\x01" \
  | curl -s "https://1.1.1.1/dns-query" \
  --data-binary @- \
  -H "Content-Type: application/dns-message" \
  -H "Accept: application/dns-message" | od -A x -t x1z -v
