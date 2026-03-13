package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

func QueryDOH(name, dnsType string) ([]string, error) {
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

	var ips []string
	for _, answer := range dnsResp.Answer {
		ips = append(ips, answer.Data)
	}

	return ips, nil
}
