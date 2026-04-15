package common

import (
	"net"
	"net/url"
	"strings"
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

// EventMatchesService reports whether a downtime event belongs to a service by
// matching either the recorded domain_name or the endpoint host to a known
// service host. Matching is exact or on a more specific subdomain.
func EventMatchesService(domainName, endpoint string, serviceHosts []string) bool {
	if len(serviceHosts) == 0 {
		return false
	}

	candidates := []string{domainName, endpoint}
	for _, candidate := range candidates {
		host := ExtractHost(candidate)
		if host == "" {
			continue
		}
		for _, serviceHost := range serviceHosts {
			if host == serviceHost || strings.HasSuffix(host, "."+serviceHost) {
				return true
			}
		}
	}

	return false
}
