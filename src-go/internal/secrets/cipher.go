// Package secrets implements project-scoped encrypted credential storage.
//
// The cipher used is AES-256-GCM. Plaintext is never persisted — only the
// (ciphertext, nonce, key_version) triple is stored. key_version is reserved
// for future master-key rotation; only version 1 is supported today.
//
// Spec reference: docs/superpowers/specs/2026-04-20-foundation-gaps-design.md
//
//	§6.1 secrets table, §11 Security Boundaries.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

const currentKeyVersion = 1

// ErrKeyVersionUnsupported is returned by Decrypt when the stored
// key_version does not match any version known to this cipher.
var ErrKeyVersionUnsupported = errors.New("secrets: unsupported key_version")

// ErrDecryptFailed is returned for any GCM authentication failure or
// ciphertext-tamper detection. We deliberately do not wrap the underlying
// crypto error so the caller cannot leak ciphertext or nonce details.
var ErrDecryptFailed = errors.New("secrets: decrypt failed")

// Cipher encrypts and decrypts secret payloads with AES-256-GCM.
// Safe for concurrent use after construction.
type Cipher struct {
	gcm cipher.AEAD
}

// NewCipher constructs a Cipher from a 32-byte master key. The key may
// be supplied as either 32 raw bytes or a 44-char base64-encoded string.
// Any other length is rejected with an error.
func NewCipher(key string) (*Cipher, error) {
	raw, err := decodeKey(key)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(raw)
	if err != nil {
		return nil, fmt.Errorf("secrets: aes init: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secrets: gcm init: %w", err)
	}
	return &Cipher{gcm: gcm}, nil
}

// Encrypt seals plaintext with a fresh random nonce. Returns the
// ciphertext, nonce, and current key_version.
func (c *Cipher) Encrypt(plaintext []byte) ([]byte, []byte, int, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, 0, fmt.Errorf("secrets: nonce: %w", err)
	}
	ct := c.gcm.Seal(nil, nonce, plaintext, nil)
	return ct, nonce, currentKeyVersion, nil
}

// Decrypt reverses Encrypt. Returns ErrKeyVersionUnsupported if version
// is unknown and ErrDecryptFailed for any other failure (including
// GCM auth tag mismatch).
func (c *Cipher) Decrypt(ciphertext, nonce []byte, version int) ([]byte, error) {
	if version != currentKeyVersion {
		return nil, ErrKeyVersionUnsupported
	}
	out, err := c.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptFailed
	}
	return out, nil
}

func decodeKey(in string) ([]byte, error) {
	if len(in) == 32 {
		return []byte(in), nil
	}
	// accept base64
	decoded, err := base64.StdEncoding.DecodeString(in)
	if err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	return nil, fmt.Errorf("secrets: AGENTFORGE_SECRETS_KEY must be 32 raw bytes or 44-char base64 (got len=%d)", len(in))
}
