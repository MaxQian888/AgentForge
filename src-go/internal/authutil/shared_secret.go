package authutil

import (
	"crypto/subtle"
	"errors"
	"strings"
)

var (
	ErrSharedSecretNotConfigured = errors.New("shared secret auth not configured")
	ErrSharedSecretMissing       = errors.New("shared secret missing")
	ErrSharedSecretInvalid       = errors.New("shared secret invalid")
)

func extractBearerToken(authHeader string) (string, error) {
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return "", ErrSharedSecretMissing
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if token == "" {
		return "", ErrSharedSecretMissing
	}
	return token, nil
}

// ValidateBearerSharedSecret checks whether the Authorization header carries
// the expected Bearer token. Empty expected secrets fail closed.
func ValidateBearerSharedSecret(authHeader, expected string) error {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return ErrSharedSecretNotConfigured
	}
	token, err := extractBearerToken(authHeader)
	if err != nil {
		return err
	}
	if len(token) != len(expected) || subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
		return ErrSharedSecretInvalid
	}
	return nil
}
