package core

import "strings"

// NormalizePlatformName converts runtime platform names into a stable source key.
func NormalizePlatformName(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = strings.TrimSuffix(normalized, "-stub")
	return normalized
}
