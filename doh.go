package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/miekg/dns"
)

type DNSQuestion struct {
	Name string `json:"name"`
	Type int    `json:"type"`
}

type DNSAnswer struct {
	Name string `json:"name"`
	Type int    `json:"type"`
	TTL  int    `json:"TTL"`
	Data string `json:"data"`
}

type DNSResponse struct {
	Status   int           `json:"Status"`
	TC       bool          `json:"TC"`
	RD       bool          `json:"RD"`
	RA       bool          `json:"RA"`
	AD       bool          `json:"AD"`
	CD       bool          `json:"CD"`
	Question []DNSQuestion `json:"Question"`
	Answer   []DNSAnswer   `json:"Answer"`
}

// DNSRecord represents a DNS answer with type and TTL
type DNSRecord struct {
	Type uint16 `json:"type"`
	TTL  uint32 `json:"ttl"`
	Data string `json:"data"`
}

func QueryDOH(name, dnsType string) ([]DNSRecord, error) {
	// Build URL safely using url.Parse
	parsedURL, err := url.Parse(cfg.DohServer)
	if err != nil {
		return nil, fmt.Errorf("invalid DoH server URL: %w", err)
	}

	// Add query parameters safely
	query := parsedURL.Query()
	query.Set("name", name)
	query.Set("type", dnsType)
	parsedURL.RawQuery = query.Encode()

	req, err := http.NewRequest("GET", parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/dns-json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var dnsResp DNSResponse
	if err := json.Unmarshal(body, &dnsResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert dnsType string to numeric type
	requestedType := dns.StringToType[dnsType]

	var records []DNSRecord
	for _, answer := range dnsResp.Answer {
		// Include requested type records AND CNAME records
		if uint16(answer.Type) == requestedType || answer.Type == 5 { // 5 = CNAME
			records = append(records, DNSRecord{
				Type: uint16(answer.Type),
				TTL:  uint32(answer.TTL),
				Data: answer.Data,
			})
		}
	}

	return records, nil
}
