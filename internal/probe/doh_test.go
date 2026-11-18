package probe

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
)

func TestCleanEndpoint(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://ru.dns-ai.ru/-token-{?dns}", "https://ru.dns-ai.ru/-token-"},
		{"https://cf.dns-ai.ru/dns-query{?dns}", "https://cf.dns-ai.ru/dns-query"},
		{"https://cf.dns-ai.ru/dns-query?", "https://cf.dns-ai.ru/dns-query"},
		{"https://cf.dns-ai.ru/dns-query?foo=bar&", "https://cf.dns-ai.ru/dns-query?foo=bar"},
		{"https://dns.dns-ai.ru/dns-query", "https://dns.dns-ai.ru/dns-query"},
	}

	for _, tc := range tests {
		actual := CleanEndpoint(tc.input)
		if actual != tc.expected {
			t.Errorf("CleanEndpoint(%q) = %q, expected %q", tc.input, actual, tc.expected)
		}
	}
}

func TestBuildDNSQuery(t *testing.T) {
	query, err := BuildDNSQuery("gemini.google.com")
	if err != nil {
		t.Fatalf("BuildDNSQuery failed: %v", err)
	}
	if len(query) < 12 {
		t.Fatalf("query packet too short: %d bytes", len(query))
	}

	// Verify Flags (RD=1)
	flags := binary.BigEndian.Uint16(query[2:4])
	if flags != 0x0100 {
		t.Errorf("expected flags 0x0100, got 0x%04x", flags)
	}

	// Verify QDCOUNT = 1
	qdcount := binary.BigEndian.Uint16(query[4:6])
	if qdcount != 1 {
		t.Errorf("expected QDCOUNT 1, got %d", qdcount)
	}
}

func TestIsFilteredGatewayIP(t *testing.T) {
	filteredCases := []string{
		"127.0.0.1",
		"10.0.0.1",
		"192.168.1.1",
		"172.16.0.1",
		"0.0.0.0",
		"142.250.190.46",  // Google
		"172.217.16.206",  // Google
		"172.64.80.1",     // Cloudflare
		"172.67.164.60",   // Cloudflare
		"173.194.222.100", // Google
		"104.18.2.1",      // Cloudflare
		"108.162.192.1",   // Cloudflare
	}

	for _, ipStr := range filteredCases {
		ip := net.ParseIP(ipStr)
		if !IsFilteredGatewayIP(ip) {
			t.Errorf("expected IP %s to be filtered out, but it was accepted", ipStr)
		}
	}

	validGateways := []string{
		"5.129.221.64",
		"45.83.27.4",
		"191.217.212.16",
		"193.233.112.88",
		"31.58.216.201",
	}

	for _, ipStr := range validGateways {
		ip := net.ParseIP(ipStr)
		if IsFilteredGatewayIP(ip) {
			t.Errorf("expected valid gateway IP %s not to be filtered, but it was rejected", ipStr)
		}
	}
}

func TestParseDNSResponse(t *testing.T) {
	// Build mock DNS response with 1 question and 2 answers:
	// Answer 1: 142.250.1.1 (filtered Google IP)
	// Answer 2: 5.129.221.64 (clean Smart DNS gateway)
	buf := new(bytes.Buffer)

	// Header: ID=0x1234, Flags=0x8180 (response, no error), QD=1, AN=2, NS=0, AR=0
	_ = binary.Write(buf, binary.BigEndian, uint16(0x1234))
	_ = binary.Write(buf, binary.BigEndian, uint16(0x8180))
	_ = binary.Write(buf, binary.BigEndian, uint16(1))
	_ = binary.Write(buf, binary.BigEndian, uint16(2))
	_ = binary.Write(buf, binary.BigEndian, uint16(0))
	_ = binary.Write(buf, binary.BigEndian, uint16(0))

	// Question: gemini.google.com, type A, class IN
	buf.WriteByte(6)
	buf.WriteString("gemini")
	buf.WriteByte(6)
	buf.WriteString("google")
	buf.WriteByte(3)
	buf.WriteString("com")
	buf.WriteByte(0)
	_ = binary.Write(buf, binary.BigEndian, uint16(1)) // A
	_ = binary.Write(buf, binary.BigEndian, uint16(1)) // IN

	// Answer 1 (Pointer to question name 0xc00c, Type A, Class IN, TTL 300, RDLEN 4, IP 142.250.1.1)
	buf.WriteByte(0xc0)
	buf.WriteByte(0x0c)
	_ = binary.Write(buf, binary.BigEndian, uint16(1))
	_ = binary.Write(buf, binary.BigEndian, uint16(1))
	_ = binary.Write(buf, binary.BigEndian, uint32(300))
	_ = binary.Write(buf, binary.BigEndian, uint16(4))
	buf.Write([]byte{142, 250, 1, 1})

	// Answer 2 (Pointer to question name 0xc00c, Type A, Class IN, TTL 300, RDLEN 4, IP 5.129.221.64)
	buf.WriteByte(0xc0)
	buf.WriteByte(0x0c)
	_ = binary.Write(buf, binary.BigEndian, uint16(1))
	_ = binary.Write(buf, binary.BigEndian, uint16(1))
	_ = binary.Write(buf, binary.BigEndian, uint32(300))
	_ = binary.Write(buf, binary.BigEndian, uint16(4))
	buf.Write([]byte{5, 129, 221, 64})

	ips, err := ParseDNSResponse(buf.Bytes())
	if err != nil {
		t.Fatalf("ParseDNSResponse failed: %v", err)
	}

	if len(ips) != 1 {
		t.Fatalf("expected 1 clean IP after filtering, got %d: %v", len(ips), ips)
	}
	if ips[0] != "5.129.221.64" {
		t.Errorf("expected 5.129.221.64, got %s", ips[0])
	}
}
