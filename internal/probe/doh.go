package probe

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"airoute/internal/domain"
)

// CleanEndpoint removes RFC 8484 URI templates like {?dns} and dangling query separators.
func CleanEndpoint(raw string) string {
	cleaned := strings.ReplaceAll(raw, "{?dns}", "")
	cleaned = strings.TrimRight(cleaned, "?&")
	return strings.TrimSpace(cleaned)
}

// BuildDNSQuery constructs a standard RFC 1035 A record query packet.
func BuildDNSQuery(domainName string) ([]byte, error) {
	var txID [2]byte
	if _, err := rand.Read(txID[:]); err != nil {
		txID = [2]byte{0x12, 0x34}
	}

	buf := new(bytes.Buffer)
	// Header: ID (2B), Flags=0x0100 (RD=1) (2B), QDCOUNT=1 (2B), ANCOUNT=0, NSCOUNT=0, ARCOUNT=0
	_ = binary.Write(buf, binary.BigEndian, txID)
	_ = binary.Write(buf, binary.BigEndian, uint16(0x0100))
	_ = binary.Write(buf, binary.BigEndian, uint16(1))
	_ = binary.Write(buf, binary.BigEndian, uint16(0))
	_ = binary.Write(buf, binary.BigEndian, uint16(0))
	_ = binary.Write(buf, binary.BigEndian, uint16(0))

	parts := strings.Split(strings.Trim(domainName, "."), ".")
	for _, part := range parts {
		if len(part) == 0 || len(part) > 63 {
			return nil, fmt.Errorf("invalid label length in domain %q", domainName)
		}
		buf.WriteByte(byte(len(part)))
		buf.WriteString(part)
	}
	buf.WriteByte(0x00) // terminating root label

	// QTYPE=A (1), QCLASS=IN (1)
	_ = binary.Write(buf, binary.BigEndian, uint16(1))
	_ = binary.Write(buf, binary.BigEndian, uint16(1))

	return buf.Bytes(), nil
}

// ParseDNSResponse parses an RFC 1035 / RFC 8484 DNS response payload, extracting IPv4 A records
// while filtering out direct BigTech / Cloudflare edge IPs and private/unusable addresses.
func ParseDNSResponse(data []byte) ([]string, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("dns response too short (%d bytes)", len(data))
	}

	flags := binary.BigEndian.Uint16(data[2:4])
	rcode := flags & 0x000F
	if rcode != 0 {
		return nil, fmt.Errorf("dns server returned rcode %d", rcode)
	}

	qdcount := int(binary.BigEndian.Uint16(data[4:6]))
	ancount := int(binary.BigEndian.Uint16(data[6:8]))

	pos := 12
	// Skip question records
	for i := 0; i < qdcount; i++ {
		for pos < len(data) {
			b := data[pos]
			if b == 0 {
				pos++
				break
			}
			if b&0xc0 == 0xc0 {
				pos += 2
				break
			}
			pos += int(b) + 1
		}
		pos += 4 // QTYPE (2) + QCLASS (2)
	}

	var ips []string
	seen := make(map[string]bool)

	// Parse answer records
	for i := 0; i < ancount && pos < len(data); i++ {
		// Skip name
		if pos < len(data) && data[pos]&0xc0 == 0xc0 {
			pos += 2
		} else {
			for pos < len(data) {
				b := data[pos]
				if b == 0 {
					pos++
					break
				}
				if b&0xc0 == 0xc0 {
					pos += 2
					break
				}
				pos += int(b) + 1
			}
		}

		if pos+10 > len(data) {
			break
		}

		rtype := binary.BigEndian.Uint16(data[pos : pos+2])
		rclass := binary.BigEndian.Uint16(data[pos+2 : pos+4])
		// ttl := binary.BigEndian.Uint32(data[pos+4 : pos+8])
		rdlen := int(binary.BigEndian.Uint16(data[pos+8 : pos+10]))
		pos += 10

		if pos+rdlen > len(data) {
			break
		}

		// A record (type 1) in IN class (1) with 4 bytes IPv4
		if rtype == 1 && rclass == 1 && rdlen == 4 {
			ip := net.IPv4(data[pos], data[pos+1], data[pos+2], data[pos+3])
			if !IsFilteredGatewayIP(ip) {
				ipStr := ip.String()
				if !seen[ipStr] {
					seen[ipStr] = true
					ips = append(ips, ipStr)
				}
			}
		}

		pos += rdlen
	}

	return ips, nil
}

// IsFilteredGatewayIP filters direct BigTech / Cloudflare IPs that indicate the DNS server
// did not proxy the domain, as well as loopback, private, multicast, or unspecified addresses.
func IsFilteredGatewayIP(ip net.IP) bool {
	ipv4 := ip.To4()
	if ipv4 == nil {
		return true
	}

	if ipv4.IsLoopback() || ipv4.IsPrivate() || ipv4.IsUnspecified() || ipv4.IsMulticast() || ipv4.IsLinkLocalUnicast() {
		return true
	}

	b0 := ipv4[0]
	b1 := ipv4[1]

	// 142.* (Google)
	if b0 == 142 {
		return true
	}
	// 172.217.* (Google)
	if b0 == 172 && b1 == 217 {
		return true
	}
	// 172.64.* - 172.67.* (Cloudflare direct edge)
	if b0 == 172 && (b1 >= 64 && b1 <= 67) {
		return true
	}
	// 173.194.* (Google)
	if b0 == 173 && b1 == 194 {
		return true
	}
	// 104.* (Cloudflare direct edge)
	if b0 == 104 {
		return true
	}
	// 108.162.* (Cloudflare direct edge)
	if b0 == 108 && b1 == 162 {
		return true
	}

	return false
}

// ResolveViaProvider executes a DNS resolution using UDP or DoH (GET-first) via the specified provider.
func ResolveViaProvider(prov domain.Provider, domainName string, timeout time.Duration) ([]string, error) {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	query, err := BuildDNSQuery(domainName)
	if err != nil {
		return nil, fmt.Errorf("build dns query: %w", err)
	}

	isHTTP := strings.HasPrefix(strings.ToLower(prov.Endpoint), "http://") ||
		strings.HasPrefix(strings.ToLower(prov.Endpoint), "https://")

	if prov.Kind == domain.KindUDP || !isHTTP {
		return resolveUDP(prov, query, timeout)
	}

	return resolveDoH(prov, query, timeout)
}

func resolveUDP(prov domain.Provider, query []byte, timeout time.Duration) ([]string, error) {
	target := prov.Endpoint
	if target == "" && len(prov.FallbackIPs) > 0 {
		target = prov.FallbackIPs[0]
	}
	if target == "" {
		return nil, fmt.Errorf("no endpoint or fallback IP for UDP provider %s", prov.Name)
	}

	if _, _, err := net.SplitHostPort(target); err != nil {
		target = net.JoinHostPort(target, "53")
	}

	conn, err := net.DialTimeout("udp", target, timeout)
	if err != nil {
		return nil, fmt.Errorf("udp dial to %s: %w", target, err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	if _, err := conn.Write(query); err != nil {
		return nil, fmt.Errorf("udp write: %w", err)
	}

	respBuf := make([]byte, 2048)
	n, err := conn.Read(respBuf)
	if err != nil {
		return nil, fmt.Errorf("udp read: %w", err)
	}

	return ParseDNSResponse(respBuf[:n])
}

func resolveDoH(prov domain.Provider, query []byte, timeout time.Duration) ([]string, error) {
	endpoint := CleanEndpoint(prov.Endpoint)
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid doh endpoint url: %w", err)
	}

	transport := &http.Transport{
		Proxy: nil, // explicitly disable environment proxies (no_proxy parity with DNS-AI)
		TLSClientConfig: &tls.Config{
			ServerName: parsedURL.Hostname(),
		},
		DisableKeepAlives: false,
		MaxIdleConns:      10,
		IdleConnTimeout:   30 * time.Second,
	}

	if len(prov.FallbackIPs) > 0 {
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			_, port, err := net.SplitHostPort(addr)
			if err != nil {
				port = "443"
			}
			dialer := &net.Dialer{Timeout: timeout}
			for _, fip := range prov.FallbackIPs {
				conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(fip, port))
				if dialErr == nil {
					return conn, nil
				}
			}
			// Fallback to system resolver
			return dialer.DialContext(ctx, network, addr)
		}
	} else {
		transport.DialContext = (&net.Dialer{Timeout: timeout}).DialContext
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	// 1. GET-first attempt with ?dns= (critical for ru.dns-ai.ru and standard RFC 8484)
	b64Query := base64.RawURLEncoding.EncodeToString(query)
	getURL := endpoint
	if strings.Contains(endpoint, "?") {
		getURL += "&dns=" + b64Query
	} else {
		getURL += "?dns=" + b64Query
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create get request: %w", err)
	}
	getReq.Header.Set("Accept", "application/dns-message")

	resp, err := client.Do(getReq)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			body, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				return nil, fmt.Errorf("read get response: %w", readErr)
			}
			return ParseDNSResponse(body)
		}
		// If 405 Method Not Allowed, fallback to POST below
		if resp.StatusCode != http.StatusMethodNotAllowed {
			return nil, fmt.Errorf("doh server returned HTTP %d", resp.StatusCode)
		}
	} else if !strings.Contains(err.Error(), "405") {
		// Network or dial error
		return nil, fmt.Errorf("doh get request failed: %w", err)
	}

	// 2. Fallback to POST when GET is rejected with 405 Method Not Allowed
	postCtx, postCancel := context.WithTimeout(context.Background(), timeout)
	defer postCancel()

	postReq, err := http.NewRequestWithContext(postCtx, http.MethodPost, endpoint, bytes.NewReader(query))
	if err != nil {
		return nil, fmt.Errorf("create post request: %w", err)
	}
	postReq.Header.Set("Content-Type", "application/dns-message")
	postReq.Header.Set("Accept", "application/dns-message")

	postResp, err := client.Do(postReq)
	if err != nil {
		return nil, fmt.Errorf("doh post request failed: %w", err)
	}
	defer postResp.Body.Close()

	if postResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("doh post returned HTTP %d", postResp.StatusCode)
	}

	body, err := io.ReadAll(postResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read post response: %w", err)
	}

	return ParseDNSResponse(body)
}
