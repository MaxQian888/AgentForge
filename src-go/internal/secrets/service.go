package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ErrSecretNotFound is the public, non-leaking error returned to callers
// when a name lookup misses. Its string form ("secret:not_found") is the
// documented spec error code and is what node-runtime surfaces back to
// the workflow author.
var ErrSecretNotFound = errors.New("secret:not_found")

// ErrSecretDecryptFailed is the public error returned when ciphertext
// cannot be decrypted (key mismatch, tamper, etc). We deliberately do
// not wrap the underlying crypto error so nothing about ciphertext or
// nonce leaks into the caller's error chain.
var ErrSecretDecryptFailed = errors.New("secret:decrypt_failed")

// AuditRecorder is the narrow seam Service uses to emit audit events.
// Implemented by an adapter that forwards into service.AuditService;
// tests substitute a synchronous in-memory recorder.
//
// payload is a small JSON document containing only safe metadata
// (`name`, `op`, optional `description`) — NEVER the secret value.
type AuditRecorder interface {
	Record(ctx context.Context, projectID uuid.UUID, action, resourceID, payload string, actor *uuid.UUID)
}

// Service orchestrates cipher + repo + audit for the secrets subsystem.
type Service struct {
	repo   Repository
	cipher *Cipher
	audit  AuditRecorder
}

// NewService wires a Service. The audit recorder may be nil for tests
// that do not want to assert audit emission.
func NewService(repo Repository, c *Cipher, audit AuditRecorder) *Service {
	return &Service{repo: repo, cipher: c, audit: audit}
}

// CreateSecret encrypts plaintext, persists the row, and emits an audit
// event. plaintext is held only for the duration of this call.
func (s *Service) CreateSecret(ctx context.Context, projectID uuid.UUID, name, plaintext, description string, actor uuid.UUID) (*Record, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	ct, nonce, ver, err := s.cipher.Encrypt([]byte(plaintext))
	if err != nil {
		return nil, fmt.Errorf("encrypt: %w", err)
	}
	rec := &Record{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Name:        name,
		Ciphertext:  ct,
		Nonce:       nonce,
		KeyVersion:  ver,
		Description: description,
		CreatedBy:   actor,
	}
	if err := s.repo.Create(ctx, rec); err != nil {
		return nil, err
	}
	s.emitAudit(ctx, projectID, "secret.create", name, description, &actor)
	return rec, nil
}

// RotateSecret replaces the ciphertext with a fresh encryption of the
// new plaintext. Description is NOT touched here — use UpdateDescription
// if you need to change it (not implemented in 1B; out of scope).
func (s *Service) RotateSecret(ctx context.Context, projectID uuid.UUID, name, plaintext string, actor uuid.UUID) error {
	existing, err := s.repo.Get(ctx, projectID, name)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrSecretNotFound
		}
		return err
	}
	ct, nonce, ver, err := s.cipher.Encrypt([]byte(plaintext))
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}
	existing.Ciphertext = ct
	existing.Nonce = nonce
	existing.KeyVersion = ver
	if err := s.repo.Update(ctx, existing); err != nil {
		return err
	}
	s.emitAudit(ctx, projectID, "secret.rotate", name, "", &actor)
	return nil
}

// DeleteSecret removes the row and emits an audit event.
func (s *Service) DeleteSecret(ctx context.Context, projectID uuid.UUID, name string, actor uuid.UUID) error {
	if _, err := s.repo.Get(ctx, projectID, name); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrSecretNotFound
		}
		return err
	}
	if err := s.repo.Delete(ctx, projectID, name); err != nil {
		return err
	}
	s.emitAudit(ctx, projectID, "secret.delete", name, "", &actor)
	return nil
}

// ListSecrets returns metadata-only rows (no ciphertext/nonce in the
// wire-bound DTOs — the handler strips those).
func (s *Service) ListSecrets(ctx context.Context, projectID uuid.UUID) ([]*Record, error) {
	return s.repo.List(ctx, projectID)
}

// Resolve returns plaintext for the secret. Used ONLY by the secret_resolver
// (workflow template engine) inside an HTTP node's outbound request path.
// Touches last_used_at as a side effect; failures of the touch are logged
// but never surfaced — they don't block the workflow.
//
// SECURITY: callers MUST NOT log, broadcast, or persist the returned
// plaintext. The secret_resolver injects it directly into the outbound
// HTTP request and discards the local variable.
func (s *Service) Resolve(ctx context.Context, projectID uuid.UUID, name string) (string, error) {
	rec, err := s.repo.Get(ctx, projectID, name)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", ErrSecretNotFound
		}
		return "", err
	}
	plain, err := s.cipher.Decrypt(rec.Ciphertext, rec.Nonce, rec.KeyVersion)
	if err != nil {
		return "", ErrSecretDecryptFailed
	}
	// Best-effort: do not block on touch failure.
	_ = s.repo.TouchLastUsed(ctx, projectID, name, time.Now().UTC())
	return string(plain), nil
}

func (s *Service) emitAudit(ctx context.Context, projectID uuid.UUID, action, name, description string, actor *uuid.UUID) {
	if s.audit == nil {
		return
	}
	payload := map[string]any{"name": name, "op": action}
	if description != "" {
		payload["description"] = description
	}
	b, _ := json.Marshal(payload)
	s.audit.Record(ctx, projectID, action, name, string(b), actor)
}

func validateName(name string) error {
	if name == "" {
		return errors.New("secret name must not be empty")
	}
	if len(name) > 128 {
		return errors.New("secret name must be <= 128 chars")
	}
	return nil
}
