package domain

import "time"

// ServiceTarget defines supported AI ecosystem targets.
type ServiceTarget string

const (
	TargetAll         ServiceTarget = "all"
	TargetGemini      ServiceTarget = "gemini"
	TargetClaude      ServiceTarget = "claude"
	TargetChatGPT     ServiceTarget = "chatgpt"
	TargetAntigravity ServiceTarget = "antigravity"
	TargetCursor      ServiceTarget = "cursor"
)

// TargetDomains maps each service target to its critical routed domain list.
var TargetDomains = map[ServiceTarget][]string{
	TargetGemini: {
		"gemini.google.com",
		"geminiweb-pa.clients6.google.com",
		"aistudio.google.com",
		"alkalimakersuite-pa.clients6.google.com",
		"proactivebackend-pa.googleapis.com",
		"generativelanguage.googleapis.com",
		"makersuite.google.com",
	},
	TargetClaude: {
		"claude.ai",
		"api.anthropic.com",
		"cdn.claude.ai",
		"anthropic.com",
		"statsig.anthropic.com",
	},
	TargetChatGPT: {
		"chatgpt.com",
		"openai.com",
		"chat.com",
		"sora.com",
		"oaistatic.com",
		"oaiusercontent.com",
		"oaistatsig.com",
		"cdn.oaistatic.com",
		"persistent.oaistatic.com",
		"api.openai.com",
		"auth.openai.com",
		"auth0.openai.com",
		"platform.openai.com",
		"ab.chatgpt.com",
		"files.oaiusercontent.com",
	},
	TargetAntigravity: {
		"daily-cloudcode-pa.googleapis.com",
		"cloudcode-pa.googleapis.com",
		"aicode.googleapis.com",
		"jetski-webchannel.googleapis.com",
		"generativelanguage.googleapis.com",
		"oauth2.googleapis.com",
	},
	TargetCursor: {
		"cursor.com",
		"api2.cursor.sh",
		"repo42.cursor.sh",
		"api.cursor.sh",
	},
}

// GetAllDomains returns the combined deduplicated list of domains for all AI targets.
func GetAllDomains() []string {
	seen := make(map[string]bool)
	var list []string
	for _, domains := range TargetDomains {
		for _, d := range domains {
			if !seen[d] {
				seen[d] = true
				list = append(list, d)
			}
		}
	}
	return list
}

// ProviderKind defines the DNS transport protocol.
type ProviderKind string

const (
	KindDoH ProviderKind = "doh"
	KindDoT ProviderKind = "dot"
	KindUDP ProviderKind = "udp"
)

// Provider represents a Smart DNS / DoH gateway provider.
type Provider struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Kind        ProviderKind `json:"kind"`
	Endpoint    string       `json:"endpoint"`
	FallbackIPs []string     `json:"fallback_ips,omitempty"`
	Gateways    []string     `json:"gateways,omitempty"`
	Enabled     bool         `json:"enabled"`
	IsCustom    bool         `json:"is_custom"`
}

// ProbeResult stores latency and health verification for a candidate IP or gateway.
type ProbeResult struct {
	ProviderID  string        `json:"provider_id"`
	ProviderName string       `json:"provider_name"`
	IP          string        `json:"ip"`
	LatencyMs   int64         `json:"latency_ms"`
	IsClean     bool          `json:"is_clean"`
	Reason      string        `json:"reason"`
	Target      ServiceTarget `json:"target"`
}

// RouteMode indicates how traffic is redirected.
type RouteMode string

const (
	ModeHosts RouteMode = "hosts" // Direct /etc/hosts or Windows hosts file (zero-overhead)
	ModeDoH   RouteMode = "doh"   // Native system DoH (Windows 11 / systemd-resolved)
	ModeProxy RouteMode = "proxy" // Embedded local HTTP CONNECT proxy
)

// RouteState represents the live status of the system routing.
type RouteState struct {
	IsEnabled      bool          `json:"is_enabled"`
	ActiveMode     RouteMode     `json:"active_mode"`
	ActiveTarget   ServiceTarget `json:"active_target"`
	ActiveProvider string        `json:"active_provider"`
	ActiveIP       string        `json:"active_ip"`
	LatencyMs      int64         `json:"latency_ms"`
	LastUpdated    time.Time     `json:"last_updated"`
	Message        string        `json:"message"`
}
