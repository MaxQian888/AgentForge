package secrets_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/react-go-quick-starter/server/internal/secrets"
)

type recordedAudit struct {
	ResourceID string
	Payload    string
}

type fakeAudit struct{ events []recordedAudit }

func (f *fakeAudit) Record(_ context.Context, projectID uuid.UUID, action, resourceID, payload string, actor *uuid.UUID) {
	_ = projectID
	_ = action
	_ = actor
	f.events = append(f.events, recordedAudit{ResourceID: resourceID, Payload: payload})
}

func newSvcUnderTest(t *testing.T) (*secrets.Service, *memRepo, *fakeAudit, *bytes.Buffer) {
	t.Helper()
	c, err := secrets.NewCipher(testKey)
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	repo := newMemRepo()
	audit := &fakeAudit{}
	// capture logs to assert no plaintext appears
	buf := &bytes.Buffer{}
	log.SetOutput(io.MultiWriter(buf))
	t.Cleanup(func() { log.SetOutput(io.Discard) })
	return secrets.NewService(repo, c, audit), repo, audit, buf
}

func TestService_CreateAndResolveTouchesLastUsed(t *testing.T) {
	svc, repo, audit, logs := newSvcUnderTest(t)
	ctx := context.Background()
	proj := uuid.New()
	actor := uuid.New()

	if _, err := svc.CreateSecret(ctx, proj, "GITHUB_TOKEN", "ghp_xyz", "review token", actor); err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(audit.events) != 1 || !strings.Contains(audit.events[0].Payload, "GITHUB_TOKEN") {
		t.Fatalf("audit not recorded with name: %+v", audit.events)
	}
	if strings.Contains(audit.events[0].Payload, "ghp_xyz") {
		t.Fatalf("audit payload leaks plaintext: %s", audit.events[0].Payload)
	}

	plain, err := svc.Resolve(ctx, proj, "GITHUB_TOKEN")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if plain != "ghp_xyz" {
		t.Errorf("expected plaintext, got %q", plain)
	}

	rec, _ := repo.Get(ctx, proj, "GITHUB_TOKEN")
	if rec.LastUsedAt == nil {
		t.Errorf("expected LastUsedAt to be set after Resolve")
	}

	if strings.Contains(logs.String(), "ghp_xyz") {
		t.Fatalf("plaintext leaked into logs: %s", logs.String())
	}
}

func TestService_RotateProducesNewCiphertext(t *testing.T) {
	svc, repo, _, _ := newSvcUnderTest(t)
	ctx := context.Background()
	proj := uuid.New()
	actor := uuid.New()
	_, _ = svc.CreateSecret(ctx, proj, "API_KEY", "v1", "", actor)
	before, _ := repo.Get(ctx, proj, "API_KEY")

	if err := svc.RotateSecret(ctx, proj, "API_KEY", "v2", actor); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	after, _ := repo.Get(ctx, proj, "API_KEY")
	if bytes.Equal(before.Ciphertext, after.Ciphertext) {
		t.Errorf("rotate did not change ciphertext")
	}

	plain, _ := svc.Resolve(ctx, proj, "API_KEY")
	if plain != "v2" {
		t.Errorf("rotate did not change plaintext: got %q", plain)
	}
}

func TestService_ResolveMissingReturnsTypedError(t *testing.T) {
	svc, _, _, _ := newSvcUnderTest(t)
	if _, err := svc.Resolve(context.Background(), uuid.New(), "missing"); err == nil {
		t.Fatal("expected error")
	} else if err.Error() != "secret:not_found" {
		t.Errorf("expected secret:not_found, got %v", err)
	}
}
