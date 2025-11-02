package common

import "strings"

// NormalizeCheckType returns the canonical textual representation of a check type.
// It maps legacy numeric values to their string equivalents and lowercases the result.
func NormalizeCheckType(raw string) string {
	switch strings.ToLower(raw) {
	case "1", "site":
		return "site"
	case "2", "domain":
		return "domain"
	case "3", "endpoint":
		return "endpoint"
	default:
		return strings.ToLower(raw)
	}
}

// ExpandCheckTypeValues returns all representations (string + numeric) that should
// be considered equivalent for the provided check type filter.
func ExpandCheckTypeValues(checkType string) []string {
	switch NormalizeCheckType(checkType) {
	case "site":
		return []string{"site", "1"}
	case "domain":
		return []string{"domain", "2"}
	case "endpoint":
		return []string{"endpoint", "3"}
	default:
		if checkType == "" {
			return nil
		}
		return []string{checkType}
	}
}

// DomainEndpointValues returns the set of values that correspond to domain or endpoint checks.
func DomainEndpointValues() []string {
	return []string{"domain", "2", "endpoint", "3"}
}

// SiteValues returns the set of values that correspond to site checks.
func SiteValues() []string {
	return []string{"site", "1"}
}
