package secrets_test

import (
	"bytes"
	"testing"

	"github.com/react-go-quick-starter/server/internal/secrets"
)

const testKey = "0123456789abcdef0123456789abcdef" // 32 raw bytes

func TestCipher_RoundTrip(t *testing.T) {
	c, err := secrets.NewCipher(testKey)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	plaintext := []byte("ghp_secret_token_value")
	ct, nonce, ver, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if ver != 1 {
		t.Errorf("expected key_version=1, got %d", ver)
	}
	if bytes.Equal(ct, plaintext) {
		t.Errorf("ciphertext must not equal plaintext")
	}
	got, err := c.Decrypt(ct, nonce, ver)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("round-trip mismatch: %q", string(got))
	}
}

func TestCipher_KeyVersionMismatch(t *testing.T) {
	c, _ := secrets.NewCipher(testKey)
	ct, nonce, _, _ := c.Encrypt([]byte("x"))
	if _, err := c.Decrypt(ct, nonce, 999); err == nil {
		t.Fatal("expected error on unknown key_version")
	}
}

func TestCipher_TamperedCiphertext(t *testing.T) {
	c, _ := secrets.NewCipher(testKey)
	ct, nonce, ver, _ := c.Encrypt([]byte("hello"))
	ct[0] ^= 0xFF
	if _, err := c.Decrypt(ct, nonce, ver); err == nil {
		t.Fatal("expected GCM auth error on tampered ciphertext")
	}
}

func TestNewCipher_RejectsShortKey(t *testing.T) {
	if _, err := secrets.NewCipher("too-short"); err == nil {
		t.Fatal("expected error for non-32-byte key")
	}
}

func TestNewCipher_AcceptsBase64Key(t *testing.T) {
	// 32 bytes encoded as 44-char base64
	const b64 = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	if _, err := secrets.NewCipher(b64); err != nil {
		t.Fatalf("expected base64 key to be accepted, got %v", err)
	}
}
