package adsplatform

import "errors"

// Sentinel error categories. Provider implementations wrap their native
// errors with these so callers can branch without provider-specific knowledge.
var (
	ErrProviderNotFound = errors.New("adsplatform: provider not registered")
	ErrAuthExpired      = errors.New("adsplatform: auth_expired")      // 401/403 from upstream
	ErrRateLimited      = errors.New("adsplatform: rate_limited")      // 429 / known throttle codes
	ErrTransientFailure = errors.New("adsplatform: transient_failure") // 5xx / network
	ErrInvalidRequest   = errors.New("adsplatform: invalid_request")   // 4xx other than auth
	ErrUpstreamRejected = errors.New("adsplatform: upstream_rejected") // platform-level business reject
)
