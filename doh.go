package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/miekg/dns"
)

// QueryDOH performs a DNS-over-HTTPS query using RFC 8484 wire format
func QueryDOH(name string, qtype uint16) (*dns.Msg, error) {
	// Create DNS query message
	m := new(dns.Msg)
	m.SetQuestion(name, qtype)
	m.RecursionDesired = true

	// Pack the message into wire format
	wireData, err := m.Pack()
	if err != nil {
		return nil, fmt.Errorf("failed to pack DNS message: %w", err)
	}

	// Create POST request with wire format
	req, err := http.NewRequest("POST", cfg.DohServer, bytes.NewReader(wireData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")

	// Send the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DoH server returned status %d", resp.StatusCode)
	}

	// Read and unpack the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	responseMsg := new(dns.Msg)
	if err := responseMsg.Unpack(body); err != nil {
		return nil, fmt.Errorf("failed to unpack DNS response: %w", err)
	}

	return responseMsg, nil
}
