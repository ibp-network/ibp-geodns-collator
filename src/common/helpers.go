package common

import (
	"net"
	"net/url"
	"strings"

	cfg "github.com/ibp-network/ibp-geodns-libs/config"
)

var unsafeFilenameChars = strings.NewReplacer(
	" ", "_",
	"/", "_",
	"\\", "_",
	":", "_",
	"*", "_",
	"?", "_",
	"\"", "_",
	"<", "_",
	">", "_",
	"|", "_",
)

// SanitizeFilename converts a human-readable label into a filesystem-safe token.
func SanitizeFilename(name string) string {
	return unsafeFilenameChars.Replace(strings.TrimSpace(name))
}

// ExtractHost normalizes URLs, host:port pairs, and raw hosts to a lowercase host value.
func ExtractHost(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	if strings.Contains(value, "://") {
		if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
			return strings.ToLower(strings.Trim(parsed.Hostname(), "[]"))
		}
	}

	if strings.Contains(value, "@") {
		parts := strings.SplitN(value, "@", 2)
		value = parts[1]
	}

	if idx := strings.Index(value, "/"); idx != -1 {
		value = value[:idx]
	}

	if host, _, err := net.SplitHostPort(value); err == nil {
		return strings.ToLower(strings.Trim(host, "[]"))
	}

	value = strings.Trim(value, "[]")
	if strings.Count(value, ":") == 1 {
		if idx := strings.LastIndex(value, ":"); idx != -1 {
			value = value[:idx]
		}
	}

	return strings.ToLower(value)
}

// NormalizeHosts lowercases, extracts, and de-duplicates service hosts.
func NormalizeHosts(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		host := ExtractHost(value)
		if host == "" {
			continue
		}
		if _, exists := seen[host]; exists {
			continue
		}
		seen[host] = struct{}{}
		normalized = append(normalized, host)
	}
	return normalized
}

// NormalizeDomainLabel canonicalizes a monitor/domain label to host or host:port.
func NormalizeDomainLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	if strings.Contains(value, "://") {
		if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
			host := strings.ToLower(strings.Trim(parsed.Hostname(), "[]"))
			port := strings.TrimSpace(parsed.Port())
			if port == "" || port == defaultPortForScheme(parsed.Scheme) {
				return host
			}
			return net.JoinHostPort(host, port)
		}
	}

	if idx := strings.Index(value, "/"); idx != -1 {
		value = value[:idx]
	}
	if strings.Contains(value, "@") {
		parts := strings.SplitN(value, "@", 2)
		value = parts[1]
	}

	if host, port, err := net.SplitHostPort(value); err == nil {
		host = strings.ToLower(strings.Trim(host, "[]"))
		if port == "" {
			return host
		}
		return net.JoinHostPort(host, port)
	}

	return strings.ToLower(strings.Trim(value, "[]"))
}

// NormalizeEndpointIdentity canonicalizes an endpoint URL so exact endpoint
// matches remain stable across default-port variations and host casing.
func NormalizeEndpointIdentity(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	if !strings.Contains(value, "://") {
		return NormalizeDomainLabel(value)
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return ""
	}

	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	host := strings.ToLower(strings.Trim(parsed.Hostname(), "[]"))
	port := strings.TrimSpace(parsed.Port())
	authority := host
	if port != "" && port != defaultPortForScheme(scheme) {
		authority = net.JoinHostPort(host, port)
	}

	path := parsed.EscapedPath()
	if parsed.RawQuery != "" {
		path += "?" + parsed.RawQuery
	}

	return scheme + "://" + authority + path
}

func defaultPortForScheme(scheme string) string {
	switch strings.ToLower(strings.TrimSpace(scheme)) {
	case "https", "wss":
		return "443"
	case "http", "ws":
		return "80"
	default:
		return ""
	}
}

// MapServiceNameByEvent returns the unique service that owns the event target.
// Exact endpoint URLs win. If only a domain label is available and that label is
// shared by multiple services, the event is treated as ambiguous and no service
// is returned.
func MapServiceNameByEvent(services map[string]cfg.Service, domainName, endpoint string) string {
	if endpointIdentity := NormalizeEndpointIdentity(endpoint); endpointIdentity != "" {
		matches := make([]string, 0, 1)
		for serviceName, service := range services {
			if serviceDefinesEndpoint(service, endpointIdentity) {
				matches = append(matches, serviceName)
			}
		}
		if len(matches) == 1 {
			return matches[0]
		}
		if len(matches) > 1 {
			return ""
		}
	}

	label := NormalizeDomainLabel(domainName)
	if label == "" && endpoint != "" {
		label = NormalizeDomainLabel(endpoint)
	}
	if label == "" {
		return ""
	}

	matches := make([]string, 0, 1)
	for serviceName, service := range services {
		if serviceDefinesDomainLabel(service, label) {
			matches = append(matches, serviceName)
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	return ""
}

// EventMatchesService reports whether an event maps uniquely to the named service.
func EventMatchesService(services map[string]cfg.Service, serviceName, domainName, endpoint string) bool {
	return MapServiceNameByEvent(services, domainName, endpoint) == serviceName
}

func serviceDefinesEndpoint(service cfg.Service, endpointIdentity string) bool {
	for _, provider := range service.Providers {
		for _, rpcURL := range provider.RpcUrls {
			if NormalizeEndpointIdentity(rpcURL) == endpointIdentity {
				return true
			}
		}
	}
	return false
}

func serviceDefinesDomainLabel(service cfg.Service, label string) bool {
	for _, provider := range service.Providers {
		for _, rpcURL := range provider.RpcUrls {
			if NormalizeDomainLabel(rpcURL) == label {
				return true
			}
		}
	}
	return false
}
