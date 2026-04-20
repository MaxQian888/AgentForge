# Spec 1B — Project-Level Secrets Store

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 落地 spec1 §6.1 / §7 / §11 — 项目级密钥库（AES-256-GCM 加密静态存储 + 一次性可见 value + 模板引擎注入 + 严格白名单）。

**Architecture:** secrets 表 + crypto/aes GCM + 三层（cipher / repo / service）后端结构 + handler 暴露 CRUD + 模板引擎扩展 `{{secrets.X}}` 仅在 HTTP 节点 config 的 headers/url_query/body 三处解析 + FE 项目密钥管理页（创建/轮换/删除，value 仅创建时一次性可见）。

**Tech Stack:** Go (crypto/aes, crypto/cipher, Echo handler), Postgres uuid + bytea, Next.js 16 App Router, Zustand, shadcn/ui.

**Depends on:** none (first wave)

**Parallel with:** 1A (Schema + dashboard) — completely independent

**Unblocks:** 1E (HTTP node uses secret_resolver)

---

## Coordination notes (read before starting)

- **Migration number**: 069 is used (Plan 1A claims 067 + 068 in parallel; 069 is next free after that). All migration references in this plan should be read as 069.
- **Audit hook**: existing audit emission goes through `service.AuditService.RecordEvent` (see `src-go/internal/service/audit_service.go`). 1B reuses that service; the new `secret.*` ActionIDs are added to the central `middleware/rbac.go` matrix so RBAC `Require()` can gate them and the audit validator accepts them. Do NOT bypass `auditSvc.RecordEvent` — it sanitizes payloads.
- **Resource type**: this plan introduces `AuditResourceTypeSecret = "secret"` to the existing enum AND extends the SQL CHECK constraint via the same migration. Without that the audit insert will be rejected at the DB layer.
- **Master key handling**: env var `AGENTFORGE_SECRETS_KEY` MUST be exactly 32 raw bytes (decoded base64 is acceptable; we accept either 32 raw chars OR a 44-char base64 string). Cipher constructor returns an error so callers (server bootstrap) can `log.Fatal` — the secrets subsystem is the only one that exits early.
- **Spec §11 invariant — plaintext lifecycle**: only `Service.Resolve` returns plaintext, and only inside the same call frame as the HTTP node's outbound request. We never write plaintext to logs, dataStore, WS, or audit payload. Tests assert this with a log capture.
- **Whitelist scope (§11)**: the secret_resolver gates `{{secrets.X}}` to a fixed set of HTTP-node config field paths. References from any other field path or from dataStore expressions reject with `secret:not_allowed_field`. The same resolver also rejects `{{system_metadata.*}}` references inside dataStore template expressions (per §14 last bullet).
- **FE project nav**: `app/(dashboard)/projects/[id]/` does NOT yet exist (only `app/(dashboard)/projects/page.tsx`). This plan creates the `[id]/secrets/page.tsx` route under a thin layout shell scoped to this slice; if a richer project-detail layout lands later it can absorb the nav entry.

---

## Task 1 — Migration 069 secrets table + audit resource_type extension

- [x] Step 1.1 — write the up migration
  - File: `src-go/migrations/069_create_secrets.up.sql`
    ```sql
    -- Project-scoped encrypted secrets store. Plaintext NEVER persisted.
    -- ciphertext + nonce produced by AES-256-GCM (see internal/secrets/cipher.go).
    -- key_version reserved for future rotation; only version 1 is supported today.
    CREATE TABLE IF NOT EXISTS secrets (
        id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        project_id    UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
        name          VARCHAR(128) NOT NULL,
        ciphertext    BYTEA NOT NULL,
        nonce         BYTEA NOT NULL,
        key_version   INT NOT NULL DEFAULT 1,
        description   TEXT,
        last_used_at  TIMESTAMPTZ,
        created_by    UUID NOT NULL,
        created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
        updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
        UNIQUE (project_id, name)
    );
    CREATE INDEX IF NOT EXISTS secrets_project_idx ON secrets(project_id);

    CREATE TRIGGER set_secrets_updated_at
        BEFORE UPDATE ON secrets
        FOR EACH ROW
        EXECUTE FUNCTION update_updated_at_column();

    -- Extend audit resource_type CHECK so secret.* events can persist.
    ALTER TABLE project_audit_events
        DROP CONSTRAINT IF EXISTS project_audit_events_resource_type_check;
    ALTER TABLE project_audit_events
        ADD CONSTRAINT project_audit_events_resource_type_check
        CHECK (resource_type IN (
            'project','member','task','team_run','workflow',
            'wiki','settings','automation','dashboard','auth',
            'invitation','secret'
        ));
    ```

- [x] Step 1.2 — write the down migration
  - File: `src-go/migrations/069_create_secrets.down.sql`
    ```sql
    DROP TRIGGER IF EXISTS set_secrets_updated_at ON secrets;
    DROP INDEX IF EXISTS secrets_project_idx;
    DROP TABLE IF EXISTS secrets;

    ALTER TABLE project_audit_events
        DROP CONSTRAINT IF EXISTS project_audit_events_resource_type_check;
    ALTER TABLE project_audit_events
        ADD CONSTRAINT project_audit_events_resource_type_check
        CHECK (resource_type IN (
            'project','member','task','team_run','workflow',
            'wiki','settings','automation','dashboard','auth','invitation'
        ));
    ```

- [x] Step 1.3 — extend audit resource type enum to include `secret`
  - File: `src-go/internal/model/audit_event.go`
  - Add at line 27 (after `AuditResourceTypeInvitation`):
    ```go
    AuditResourceTypeSecret     = "secret"
    ```
  - In `IsValidAuditResourceType` (line 30), append `AuditResourceTypeSecret` to the matched case list so `case ... AuditResourceTypeAuth, AuditResourceTypeInvitation, AuditResourceTypeSecret:`.

- [x] Step 1.4 — verify
  - Run `rtk go test ./internal/model/...` to confirm enum update compiles + existing audit-event tests pass.
  - Apply migration locally: `rtk pnpm dev:backend:restart go-orchestrator` and confirm logs show `069_create_secrets.up.sql` applied without error.

---

## Task 2 — `internal/secrets/cipher.go` AES-256-GCM wrapper

- [x] Step 2.1 — write failing cipher tests
  - File: `src-go/internal/secrets/cipher_test.go`
    ```go
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
    ```
  - Run `rtk go test ./internal/secrets/...` — fails (no package).

- [x] Step 2.2 — implement the cipher
  - File: `src-go/internal/secrets/cipher.go`
    ```go
    // Package secrets implements project-scoped encrypted credential storage.
    //
    // The cipher used is AES-256-GCM. Plaintext is never persisted — only the
    // (ciphertext, nonce, key_version) triple is stored. key_version is reserved
    // for future master-key rotation; only version 1 is supported today.
    //
    // Spec reference: docs/superpowers/specs/2026-04-20-foundation-gaps-design.md
    //   §6.1 secrets table, §11 Security Boundaries.
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
    ```
  - Run `rtk go test ./internal/secrets/...` — all five tests pass.

- [x] Step 2.3 — commit: `feat(secrets): add AES-256-GCM cipher with key_version handshake`

---

## Task 3 — `internal/secrets/repo.go` persistence layer

- [x] Step 3.1 — write failing repo unit test
  - File: `src-go/internal/secrets/repo_test.go`
    ```go
    package secrets_test

    import (
        "context"
        "testing"
        "time"

        "github.com/google/uuid"
        "github.com/react-go-quick-starter/server/internal/secrets"
    )

    // memRepo is the in-memory test double used here AND by service_test.go.
    // Real DB coverage lives in repo_integration_test.go (build tag).
    type memRepo struct{ rows map[string]*secrets.Record }

    func newMemRepo() *memRepo { return &memRepo{rows: map[string]*secrets.Record{}} }

    func key(p uuid.UUID, n string) string { return p.String() + "|" + n }

    func (m *memRepo) Create(_ context.Context, r *secrets.Record) error {
        if _, ok := m.rows[key(r.ProjectID, r.Name)]; ok {
            return secrets.ErrNameConflict
        }
        cp := *r
        m.rows[key(r.ProjectID, r.Name)] = &cp
        return nil
    }
    func (m *memRepo) Get(_ context.Context, p uuid.UUID, n string) (*secrets.Record, error) {
        r, ok := m.rows[key(p, n)]
        if !ok {
            return nil, secrets.ErrNotFound
        }
        cp := *r
        return &cp, nil
    }
    func (m *memRepo) List(_ context.Context, p uuid.UUID) ([]*secrets.Record, error) {
        var out []*secrets.Record
        for _, r := range m.rows {
            if r.ProjectID == p {
                cp := *r
                out = append(out, &cp)
            }
        }
        return out, nil
    }
    func (m *memRepo) Update(_ context.Context, r *secrets.Record) error {
        k := key(r.ProjectID, r.Name)
        if _, ok := m.rows[k]; !ok {
            return secrets.ErrNotFound
        }
        cp := *r
        m.rows[k] = &cp
        return nil
    }
    func (m *memRepo) Delete(_ context.Context, p uuid.UUID, n string) error {
        delete(m.rows, key(p, n))
        return nil
    }
    func (m *memRepo) TouchLastUsed(_ context.Context, p uuid.UUID, n string, when time.Time) error {
        r, ok := m.rows[key(p, n)]
        if !ok {
            return secrets.ErrNotFound
        }
        r.LastUsedAt = &when
        return nil
    }

    func TestMemRepoContractCompiles(t *testing.T) {
        var _ secrets.Repository = newMemRepo()
    }
    ```
  - Run `rtk go test ./internal/secrets/...` — fails (`secrets.Record`, `Repository`, `ErrNotFound`, `ErrNameConflict` undefined).

- [x] Step 3.2 — implement record + repo interface + GORM impl
  - File: `src-go/internal/secrets/repo.go`
    ```go
    package secrets

    import (
        "context"
        "errors"
        "time"

        "github.com/google/uuid"
        "github.com/jackc/pgx/v5/pgconn"
        "gorm.io/gorm"
    )

    // ErrNotFound is returned when a (project_id, name) lookup misses.
    var ErrNotFound = errors.New("secrets: not found")

    // ErrNameConflict is returned when Create violates the
    // (project_id, name) uniqueness constraint.
    var ErrNameConflict = errors.New("secrets: name already exists in project")

    // Record is the persisted row. Plaintext is NEVER held here.
    type Record struct {
        ID          uuid.UUID
        ProjectID   uuid.UUID
        Name        string
        Ciphertext  []byte
        Nonce       []byte
        KeyVersion  int
        Description string
        LastUsedAt  *time.Time
        CreatedBy   uuid.UUID
        CreatedAt   time.Time
        UpdatedAt   time.Time
    }

    // Repository is the persistence contract used by Service.
    type Repository interface {
        Create(ctx context.Context, r *Record) error
        Get(ctx context.Context, projectID uuid.UUID, name string) (*Record, error)
        List(ctx context.Context, projectID uuid.UUID) ([]*Record, error)
        Update(ctx context.Context, r *Record) error
        Delete(ctx context.Context, projectID uuid.UUID, name string) error
        TouchLastUsed(ctx context.Context, projectID uuid.UUID, name string, when time.Time) error
    }

    // ---------------- GORM-backed implementation ----------------

    type secretRecord struct {
        ID          uuid.UUID  `gorm:"column:id;primaryKey"`
        ProjectID   uuid.UUID  `gorm:"column:project_id"`
        Name        string     `gorm:"column:name"`
        Ciphertext  []byte     `gorm:"column:ciphertext"`
        Nonce       []byte     `gorm:"column:nonce"`
        KeyVersion  int        `gorm:"column:key_version"`
        Description string     `gorm:"column:description"`
        LastUsedAt  *time.Time `gorm:"column:last_used_at"`
        CreatedBy   uuid.UUID  `gorm:"column:created_by"`
        CreatedAt   time.Time  `gorm:"column:created_at"`
        UpdatedAt   time.Time  `gorm:"column:updated_at"`
    }

    func (secretRecord) TableName() string { return "secrets" }

    func toRecord(r *secretRecord) *Record {
        return &Record{
            ID: r.ID, ProjectID: r.ProjectID, Name: r.Name,
            Ciphertext: r.Ciphertext, Nonce: r.Nonce, KeyVersion: r.KeyVersion,
            Description: r.Description, LastUsedAt: r.LastUsedAt,
            CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
        }
    }

    func fromRecord(r *Record) *secretRecord {
        return &secretRecord{
            ID: r.ID, ProjectID: r.ProjectID, Name: r.Name,
            Ciphertext: r.Ciphertext, Nonce: r.Nonce, KeyVersion: r.KeyVersion,
            Description: r.Description, LastUsedAt: r.LastUsedAt,
            CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
        }
    }

    // GormRepo is the production Repository implementation.
    type GormRepo struct{ db *gorm.DB }

    // NewGormRepo wires a Repository on top of the shared GORM DB.
    func NewGormRepo(db *gorm.DB) *GormRepo { return &GormRepo{db: db} }

    func (r *GormRepo) Create(ctx context.Context, rec *Record) error {
        if rec.ID == uuid.Nil {
            rec.ID = uuid.New()
        }
        now := time.Now().UTC()
        if rec.CreatedAt.IsZero() {
            rec.CreatedAt = now
        }
        rec.UpdatedAt = now
        if err := r.db.WithContext(ctx).Create(fromRecord(rec)).Error; err != nil {
            var pgErr *pgconn.PgError
            if errors.As(err, &pgErr) && pgErr.Code == "23505" {
                return ErrNameConflict
            }
            return err
        }
        return nil
    }

    func (r *GormRepo) Get(ctx context.Context, projectID uuid.UUID, name string) (*Record, error) {
        var row secretRecord
        if err := r.db.WithContext(ctx).
            Where("project_id = ? AND name = ?", projectID, name).
            First(&row).Error; err != nil {
            if errors.Is(err, gorm.ErrRecordNotFound) {
                return nil, ErrNotFound
            }
            return nil, err
        }
        return toRecord(&row), nil
    }

    func (r *GormRepo) List(ctx context.Context, projectID uuid.UUID) ([]*Record, error) {
        var rows []secretRecord
        if err := r.db.WithContext(ctx).
            Where("project_id = ?", projectID).
            Order("name ASC").
            Find(&rows).Error; err != nil {
            return nil, err
        }
        out := make([]*Record, 0, len(rows))
        for i := range rows {
            out = append(out, toRecord(&rows[i]))
        }
        return out, nil
    }

    func (r *GormRepo) Update(ctx context.Context, rec *Record) error {
        rec.UpdatedAt = time.Now().UTC()
        res := r.db.WithContext(ctx).
            Model(&secretRecord{}).
            Where("project_id = ? AND name = ?", rec.ProjectID, rec.Name).
            Updates(map[string]any{
                "ciphertext":  rec.Ciphertext,
                "nonce":       rec.Nonce,
                "key_version": rec.KeyVersion,
                "description": rec.Description,
                "updated_at":  rec.UpdatedAt,
            })
        if res.Error != nil {
            return res.Error
        }
        if res.RowsAffected == 0 {
            return ErrNotFound
        }
        return nil
    }

    func (r *GormRepo) Delete(ctx context.Context, projectID uuid.UUID, name string) error {
        return r.db.WithContext(ctx).
            Where("project_id = ? AND name = ?", projectID, name).
            Delete(&secretRecord{}).Error
    }

    func (r *GormRepo) TouchLastUsed(ctx context.Context, projectID uuid.UUID, name string, when time.Time) error {
        res := r.db.WithContext(ctx).
            Model(&secretRecord{}).
            Where("project_id = ? AND name = ?", projectID, name).
            Update("last_used_at", when)
        if res.Error != nil {
            return res.Error
        }
        if res.RowsAffected == 0 {
            return ErrNotFound
        }
        return nil
    }
    ```
  - Run `rtk go test ./internal/secrets/...` — passes (mem repo satisfies the interface; GORM impl is exercised by integration test in Task 9).

- [x] Step 3.3 — commit: `feat(secrets): persist records via GORM repo with name-conflict mapping`

---

## Task 4 — `internal/secrets/service.go` orchestrating cipher + repo + audit

- [x] Step 4.1 — write failing service test (no plaintext leaks + last_used_at touched)
  - File: `src-go/internal/secrets/service_test.go`
    ```go
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
    ```
  - Run `rtk go test ./internal/secrets/...` — fails: `Service`, `NewService`, `AuditRecorder` undefined.

- [x] Step 4.2 — implement service
  - File: `src-go/internal/secrets/service.go`
    ```go
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

    // NewService wires a Service. All three dependencies are required.
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
    ```
  - Run `rtk go test ./internal/secrets/...` — three service tests pass.

- [x] Step 4.3 — commit: `feat(secrets): orchestrate create/rotate/resolve with audit hook`

---

## Task 5 — RBAC ActionIDs + audit-recorder adapter

- [x] Step 5.1 — extend RBAC matrix with `secret.read|write`
  - File: `src-go/internal/middleware/rbac.go`
  - Add at line 116 (after `ActionAuditRead`):
    ```go
    // Project-scoped secrets store. Read = list metadata only (no values);
    // write covers create, rotate, delete.
    ActionSecretRead  ActionID = "secret.read"
    ActionSecretWrite ActionID = "secret.write"
    ```
  - In the matrix literal (line 135 onward), add:
    ```go
    ActionSecretRead:  model.ProjectRoleViewer,
    ActionSecretWrite: model.ProjectRoleEditor,
    ```

- [x] Step 5.2 — write audit-recorder adapter test
  - File: `src-go/internal/secrets/audit_adapter_test.go`
    ```go
    package secrets_test

    import (
        "context"
        "testing"

        "github.com/google/uuid"

        "github.com/react-go-quick-starter/server/internal/model"
        "github.com/react-go-quick-starter/server/internal/secrets"
    )

    type capturedSink struct{ events []*model.AuditEvent }

    func (c *capturedSink) Record(_ context.Context, e *model.AuditEvent) error {
        c.events = append(c.events, e)
        return nil
    }

    func TestAuditServiceAdapter_EmitsResourceTypeSecret(t *testing.T) {
        sink := &capturedSink{}
        rec := secrets.NewAuditServiceAdapter(sink)

        proj := uuid.New()
        actor := uuid.New()
        rec.Record(context.Background(), proj, "secret.create", "GITHUB_TOKEN", `{"name":"GITHUB_TOKEN","op":"secret.create"}`, &actor)

        if len(sink.events) != 1 {
            t.Fatalf("expected 1 event, got %d", len(sink.events))
        }
        ev := sink.events[0]
        if ev.ResourceType != model.AuditResourceTypeSecret {
            t.Errorf("resource_type = %s want secret", ev.ResourceType)
        }
        if ev.ActionID != "secret.create" {
            t.Errorf("action_id = %s", ev.ActionID)
        }
        if ev.ResourceID != "GITHUB_TOKEN" {
            t.Errorf("resource_id = %s", ev.ResourceID)
        }
    }
    ```
  - Run `rtk go test ./internal/secrets/...` — fails (`NewAuditServiceAdapter` undefined).

- [x] Step 5.3 — implement adapter forwarding into `service.AuditService`
  - File: `src-go/internal/secrets/audit_adapter.go`
    ```go
    package secrets

    import (
        "context"
        "time"

        "github.com/google/uuid"

        "github.com/react-go-quick-starter/server/internal/model"
    )

    // AuditEventEmitter is the narrow contract auditServiceAdapter needs.
    // service.AuditService satisfies it via RecordEvent.
    type AuditEventEmitter interface {
        Record(ctx context.Context, e *model.AuditEvent) error
    }

    type auditServiceAdapter struct{ sink AuditEventEmitter }

    // NewAuditServiceAdapter wraps an AuditEventEmitter so it implements
    // AuditRecorder for the secrets Service.
    func NewAuditServiceAdapter(sink AuditEventEmitter) AuditRecorder {
        return &auditServiceAdapter{sink: sink}
    }

    func (a *auditServiceAdapter) Record(ctx context.Context, projectID uuid.UUID, action, resourceID, payload string, actor *uuid.UUID) {
        if a.sink == nil {
            return
        }
        ev := &model.AuditEvent{
            ProjectID:           projectID,
            OccurredAt:          time.Now().UTC(),
            ActorUserID:         actor,
            ActionID:            action,
            ResourceType:        model.AuditResourceTypeSecret,
            ResourceID:          resourceID,
            PayloadSnapshotJSON: payload,
        }
        _ = a.sink.Record(ctx, ev)
    }
    ```
  - Note: the production wiring (Task 8) injects a thin shim that calls `auditSvc.RecordEvent(ctx, e)` and adapts the return signature.
  - Run `rtk go test ./internal/secrets/... ./internal/middleware/...` — all pass.

- [x] Step 5.4 — commit: `feat(secrets): wire RBAC matrix entries and audit-service adapter`

---

## Task 6 — Template engine extension `secret_resolver.go` with strict whitelist

- [x] Step 6.1 — write failing resolver tests
  - File: `src-go/internal/workflow/template/secret_resolver_test.go`
    ```go
    package template_test

    import (
        "context"
        "errors"
        "testing"

        "github.com/google/uuid"

        "github.com/react-go-quick-starter/server/internal/workflow/template"
    )

    type fakeResolver struct {
        secretsByName map[string]string
    }

    func (f *fakeResolver) Resolve(_ context.Context, _ uuid.UUID, name string) (string, error) {
        v, ok := f.secretsByName[name]
        if !ok {
            return "", errors.New("secret:not_found")
        }
        return v, nil
    }

    func TestRender_AllowedFieldSubstitutesSecret(t *testing.T) {
        r := template.NewSecretResolver(&fakeResolver{secretsByName: map[string]string{"TOKEN": "ghp_xyz"}})
        out, err := r.Render(context.Background(), uuid.New(), template.FieldHTTPHeaders, "Bearer {{secrets.TOKEN}}", nil)
        if err != nil {
            t.Fatalf("render: %v", err)
        }
        if out != "Bearer ghp_xyz" {
            t.Errorf("expected substitution, got %q", out)
        }
    }

    func TestRender_DisallowedFieldRejects(t *testing.T) {
        r := template.NewSecretResolver(&fakeResolver{secretsByName: map[string]string{"TOKEN": "ghp"}})
        _, err := r.Render(context.Background(), uuid.New(), template.FieldGeneric, "Bearer {{secrets.TOKEN}}", nil)
        if !errors.Is(err, template.ErrSecretFieldNotAllowed) {
            t.Errorf("expected ErrSecretFieldNotAllowed, got %v", err)
        }
    }

    func TestRender_DataStoreReferenceStillResolvedNormally(t *testing.T) {
        r := template.NewSecretResolver(nil)
        ds := map[string]any{"node1": map[string]any{"value": "ok"}}
        out, err := r.Render(context.Background(), uuid.New(), template.FieldHTTPBody, "x={{node1.value}}", ds)
        if err != nil {
            t.Fatalf("render: %v", err)
        }
        if out != "x=ok" {
            t.Errorf("expected dataStore substitution, got %q", out)
        }
    }

    func TestRender_RejectsSystemMetadataReferenceFromAuthorCode(t *testing.T) {
        r := template.NewSecretResolver(nil)
        ds := map[string]any{"system_metadata": map[string]any{"reply_target": "x"}}
        _, err := r.Render(context.Background(), uuid.New(), template.FieldHTTPBody, "x={{system_metadata.reply_target}}", ds)
        if !errors.Is(err, template.ErrSystemMetadataNotAllowed) {
            t.Errorf("expected ErrSystemMetadataNotAllowed, got %v", err)
        }
    }

    func TestValidateConfig_RejectsSecretInDisallowedField(t *testing.T) {
        // Save-time defense in depth.
        err := template.ValidateNoSecretReferences(template.FieldGeneric, "{{secrets.X}}")
        if !errors.Is(err, template.ErrSecretFieldNotAllowed) {
            t.Errorf("expected reject at save time, got %v", err)
        }
    }

    func TestValidateConfig_AllowsSecretInHTTPHeaders(t *testing.T) {
        if err := template.ValidateNoSecretReferences(template.FieldHTTPHeaders, "Bearer {{secrets.X}}"); err != nil {
            t.Errorf("expected accept at save time, got %v", err)
        }
    }
    ```
  - Run `rtk go test ./internal/workflow/template/...` — fails (package missing).

- [x] Step 6.2 — implement the resolver
  - File: `src-go/internal/workflow/template/secret_resolver.go`
    ```go
    // Package template extends the workflow template engine with two
    // secrecy-sensitive primitives:
    //
    //   1. Substituting `{{secrets.NAME}}` references — but ONLY inside the
    //      strict whitelist of HTTP-node config fields documented in
    //      docs/superpowers/specs/2026-04-20-foundation-gaps-design.md §11.
    //   2. Rejecting `{{system_metadata.*}}` references inside dataStore
    //      template expressions, per spec §14 last bullet — author code
    //      cannot read system_metadata.
    //
    // The package intentionally has no transitive imports of crypto or repo
    // code: all secret access is delegated through the SecretSource interface
    // so the resolver can be unit-tested without DB or cipher.
    package template

    import (
        "context"
        "errors"
        "fmt"
        "regexp"
        "strings"

        "github.com/google/uuid"

        "github.com/react-go-quick-starter/server/internal/workflow/nodetypes"
    )

    // FieldKind is the structural identifier the caller passes when invoking
    // Render or ValidateNoSecretReferences. The whitelist is keyed by this
    // enum, NOT by free-form strings, so accidental typos in the HTTP node
    // implementation cannot widen the secret-injection surface.
    type FieldKind string

    const (
        // FieldHTTPHeaders is the HTTP node's headers map (key + value).
        FieldHTTPHeaders FieldKind = "http.headers"
        // FieldHTTPURLQuery is the HTTP node's url-query map (key + value).
        FieldHTTPURLQuery FieldKind = "http.url_query"
        // FieldHTTPBody is the HTTP node's request body (raw or templated).
        FieldHTTPBody FieldKind = "http.body"
        // FieldGeneric is every other DAG node config field. Secret refs
        // here are rejected.
        FieldGeneric FieldKind = "generic"
    )

    // ErrSecretFieldNotAllowed is returned when a `{{secrets.X}}` reference
    // appears in a field outside the allowlist.
    var ErrSecretFieldNotAllowed = errors.New("secret:not_allowed_field")

    // ErrSystemMetadataNotAllowed is returned when author template code
    // references `{{system_metadata.*}}`.
    var ErrSystemMetadataNotAllowed = errors.New("template:system_metadata_not_allowed")

    var secretRefRe = regexp.MustCompile(`\{\{\s*secrets\.([A-Za-z0-9_]+)\s*\}\}`)
    var systemMetadataRe = regexp.MustCompile(`\{\{\s*system_metadata(\.|\b)`)

    var allowedSecretFields = map[FieldKind]bool{
        FieldHTTPHeaders:  true,
        FieldHTTPURLQuery: true,
        FieldHTTPBody:     true,
    }

    // SecretSource is the narrow secrets-access seam. The production binding
    // is `secrets.Service.Resolve` exposed via a thin adapter.
    type SecretSource interface {
        Resolve(ctx context.Context, projectID uuid.UUID, name string) (string, error)
    }

    // SecretResolver renders templated strings, substituting allowed
    // `{{secrets.X}}` references and falling back to the existing dataStore
    // resolver for `{{node.path}}` references.
    type SecretResolver struct {
        src SecretSource
    }

    // NewSecretResolver wires a resolver. src may be nil if the caller only
    // intends to use Render for fields that contain dataStore refs only —
    // any actual `{{secrets.X}}` reference with src=nil yields an error.
    func NewSecretResolver(src SecretSource) *SecretResolver {
        return &SecretResolver{src: src}
    }

    // Render returns the input string with both secret refs and dataStore
    // refs resolved. field controls whether secret refs are accepted.
    //
    // Order of operations:
    //   1. Reject disallowed `{{system_metadata.*}}` references first
    //      (defense in depth — same rule applies to every field kind).
    //   2. Reject `{{secrets.X}}` if field is not in the allowlist.
    //   3. Substitute `{{secrets.X}}` via SecretSource.
    //   4. Hand the remaining template string to nodetypes.ResolveTemplateVars
    //      so existing `{{node.path}}` references continue to work.
    func (r *SecretResolver) Render(ctx context.Context, projectID uuid.UUID, field FieldKind, in string, dataStore map[string]any) (string, error) {
        if systemMetadataRe.MatchString(in) {
            return "", ErrSystemMetadataNotAllowed
        }
        if secretRefRe.MatchString(in) && !allowedSecretFields[field] {
            return "", ErrSecretFieldNotAllowed
        }

        var renderErr error
        rendered := secretRefRe.ReplaceAllStringFunc(in, func(match string) string {
            if renderErr != nil {
                return match
            }
            sub := secretRefRe.FindStringSubmatch(match)
            if len(sub) < 2 {
                return match
            }
            if r.src == nil {
                renderErr = errors.New("secret:not_found")
                return match
            }
            v, err := r.src.Resolve(ctx, projectID, sub[1])
            if err != nil {
                renderErr = err
                return match
            }
            return v
        })
        if renderErr != nil {
            return "", renderErr
        }

        // Hand off the (now secret-free) string to the existing template
        // engine for dataStore reference substitution.
        return nodetypes.ResolveTemplateVars(rendered, dataStore), nil
    }

    // ValidateNoSecretReferences is the save-time guard. The workflow save
    // path walks every field in every node's config and calls this with the
    // appropriate FieldKind. Returns nil when the field is allowed to host
    // secret refs OR when no secret refs are present.
    func ValidateNoSecretReferences(field FieldKind, in string) error {
        if systemMetadataRe.MatchString(in) {
            return ErrSystemMetadataNotAllowed
        }
        if !secretRefRe.MatchString(in) {
            return nil
        }
        if !allowedSecretFields[field] {
            return fmt.Errorf("%w: field %q does not permit secret references", ErrSecretFieldNotAllowed, field)
        }
        return nil
    }

    // Ensure the package keeps a stable string identity for callers that
    // log or audit field kinds.
    func (f FieldKind) String() string { return string(f) }

    // Ensure no static-init footgun: a future test can stub strings.NewReplacer
    // — we import strings only because the build will sometimes complain
    // when no usage remains; this no-op keeps the import explicit.
    var _ = strings.TrimSpace
    ```
  - Run `rtk go test ./internal/workflow/template/...` — all six tests pass.

- [x] Step 6.3 — document the contract for 1E
  - Add a comment block at the top of `src-go/internal/workflow/template/secret_resolver.go` (or a small `doc.go`) noting:
    > **API contract for HTTP node (Plan 1E)**: at execution time, for every header value, every url-query value, and the request body, call `resolver.Render(ctx, projectID, FieldHTTP{Headers,URLQuery,Body}, raw, dataStore)`. For every other config field, call `resolver.Render(ctx, projectID, FieldGeneric, raw, dataStore)`. At save time, call `ValidateNoSecretReferences(field, raw)` once per field; reject the workflow save with HTTP 400 if any field returns `ErrSecretFieldNotAllowed`.

- [x] Step 6.4 — commit: `feat(workflow): secret_resolver with strict HTTP-node field whitelist`

---

## Task 7 — `handler/secrets_handler.go` HTTP CRUD

- [ ] Step 7.1 — write failing handler test
  - File: `src-go/internal/handler/secrets_handler_test.go`
    ```go
    package handler_test

    import (
        "encoding/json"
        "net/http"
        "net/http/httptest"
        "strings"
        "testing"

        "github.com/google/uuid"
        "github.com/labstack/echo/v4"

        "github.com/react-go-quick-starter/server/internal/handler"
        appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
        "github.com/react-go-quick-starter/server/internal/secrets"
    )

    type fakeSvc struct{ stored map[string]string }

    func (f *fakeSvc) CreateSecret(_ echo.Context, projectID uuid.UUID, name, plaintext, description string, actor uuid.UUID) (*secrets.Record, error) {
        f.stored[name] = plaintext
        return &secrets.Record{ProjectID: projectID, Name: name, Description: description}, nil
    }
    func (f *fakeSvc) RotateSecret(_ echo.Context, projectID uuid.UUID, name, plaintext string, actor uuid.UUID) error {
        if _, ok := f.stored[name]; !ok {
            return secrets.ErrSecretNotFound
        }
        f.stored[name] = plaintext
        return nil
    }
    func (f *fakeSvc) DeleteSecret(_ echo.Context, projectID uuid.UUID, name string, actor uuid.UUID) error {
        delete(f.stored, name)
        return nil
    }
    func (f *fakeSvc) ListSecrets(_ echo.Context, projectID uuid.UUID) ([]*secrets.Record, error) {
        out := []*secrets.Record{}
        for n := range f.stored {
            out = append(out, &secrets.Record{ProjectID: projectID, Name: n})
        }
        return out, nil
    }

    func TestSecretsHandler_CreateReturnsValueOnce(t *testing.T) {
        e := echo.New()
        svc := &fakeSvc{stored: map[string]string{}}
        h := handler.NewSecretsHandler(svc)

        body := `{"name":"GITHUB_TOKEN","value":"ghp_xyz","description":"review token"}`
        req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/123/secrets", strings.NewReader(body))
        req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
        rec := httptest.NewRecorder()
        c := e.NewContext(req, rec)
        c.Set(appMiddleware.ProjectIDContextKey, uuid.New())
        // simulate JWT claims for actor id
        c.Set("user", &fakeJWT{userID: uuid.New().String()})

        if err := h.Create(c); err != nil {
            t.Fatalf("Create: %v", err)
        }
        if rec.Code != http.StatusCreated {
            t.Fatalf("status: %d", rec.Code)
        }
        var resp map[string]any
        _ = json.Unmarshal(rec.Body.Bytes(), &resp)
        if resp["value"] != "ghp_xyz" {
            t.Errorf("expected value echoed once, got %+v", resp)
        }
    }

    func TestSecretsHandler_ListNeverReturnsValues(t *testing.T) {
        e := echo.New()
        svc := &fakeSvc{stored: map[string]string{"TOKEN": "ghp_xyz"}}
        h := handler.NewSecretsHandler(svc)

        req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/123/secrets", nil)
        rec := httptest.NewRecorder()
        c := e.NewContext(req, rec)
        c.Set(appMiddleware.ProjectIDContextKey, uuid.New())

        if err := h.List(c); err != nil {
            t.Fatalf("List: %v", err)
        }
        if strings.Contains(rec.Body.String(), "ghp_xyz") {
            t.Fatalf("list response leaked value: %s", rec.Body.String())
        }
    }
    ```
  - Note: the test imports a tiny `fakeJWT` helper colocated with other handler tests (see existing `handler/employee_handler_test.go` for the pattern); add it here if it doesn't exist:
    ```go
    type fakeJWT struct{ userID string }
    func (f *fakeJWT) Claims(c any) error { return nil }
    ```
    (If a richer JWT extractor exists, follow that pattern instead — search `func TestEmployeeHandler` to confirm.)
  - Run `rtk go test ./internal/handler/... -run Secrets` — fails (no handler).

- [ ] Step 7.2 — implement the handler
  - File: `src-go/internal/handler/secrets_handler.go`
    ```go
    package handler

    import (
        "errors"
        "net/http"

        "github.com/google/uuid"
        "github.com/labstack/echo/v4"

        appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
        "github.com/react-go-quick-starter/server/internal/secrets"
    )

    // secretsService is the narrow contract SecretsHandler needs. echo.Context
    // is passed through so the implementation can derive cancellation +
    // request-scoped values without a separate context.Context arg.
    type secretsService interface {
        CreateSecret(c echo.Context, projectID uuid.UUID, name, plaintext, description string, actor uuid.UUID) (*secrets.Record, error)
        RotateSecret(c echo.Context, projectID uuid.UUID, name, plaintext string, actor uuid.UUID) error
        DeleteSecret(c echo.Context, projectID uuid.UUID, name string, actor uuid.UUID) error
        ListSecrets(c echo.Context, projectID uuid.UUID) ([]*secrets.Record, error)
    }

    // SecretsHandler exposes project-scoped CRUD for encrypted secrets.
    // Plaintext is returned exactly once on Create + Rotate. List/Get never
    // return values.
    type SecretsHandler struct{ svc secretsService }

    // NewSecretsHandler returns a handler backed by the given service.
    func NewSecretsHandler(svc secretsService) *SecretsHandler {
        return &SecretsHandler{svc: svc}
    }

    // Register attaches secret routes under a project-scoped Echo group
    // already protected by ProjectMiddleware + JWT.
    func (h *SecretsHandler) Register(g *echo.Group) {
        g.GET("/secrets", h.List, appMiddleware.Require(appMiddleware.ActionSecretRead))
        g.POST("/secrets", h.Create, appMiddleware.Require(appMiddleware.ActionSecretWrite))
        g.PATCH("/secrets/:name", h.Rotate, appMiddleware.Require(appMiddleware.ActionSecretWrite))
        g.DELETE("/secrets/:name", h.Delete, appMiddleware.Require(appMiddleware.ActionSecretWrite))
    }

    type secretMetadataDTO struct {
        Name        string  `json:"name"`
        Description string  `json:"description,omitempty"`
        LastUsedAt  *string `json:"lastUsedAt,omitempty"`
        CreatedBy   string  `json:"createdBy"`
        CreatedAt   string  `json:"createdAt"`
        UpdatedAt   string  `json:"updatedAt"`
    }

    func toMetadataDTO(r *secrets.Record) secretMetadataDTO {
        const layout = "2006-01-02T15:04:05Z07:00"
        dto := secretMetadataDTO{
            Name:        r.Name,
            Description: r.Description,
            CreatedBy:   r.CreatedBy.String(),
            CreatedAt:   r.CreatedAt.UTC().Format(layout),
            UpdatedAt:   r.UpdatedAt.UTC().Format(layout),
        }
        if r.LastUsedAt != nil {
            s := r.LastUsedAt.UTC().Format(layout)
            dto.LastUsedAt = &s
        }
        return dto
    }

    // List handles GET /api/v1/projects/:pid/secrets — metadata only.
    func (h *SecretsHandler) List(c echo.Context) error {
        projectID := appMiddleware.GetProjectID(c)
        rows, err := h.svc.ListSecrets(c, projectID)
        if err != nil {
            return c.JSON(http.StatusInternalServerError, map[string]string{"error": "secret:list_failed"})
        }
        out := make([]secretMetadataDTO, 0, len(rows))
        for _, r := range rows {
            out = append(out, toMetadataDTO(r))
        }
        return c.JSON(http.StatusOK, out)
    }

    type createSecretReq struct {
        Name        string `json:"name"`
        Value       string `json:"value"`
        Description string `json:"description,omitempty"`
    }

    type createSecretResp struct {
        secretMetadataDTO
        // Value is returned ONCE on the create response. The FE shows it
        // behind a "copy + acknowledge" UI and discards it from memory.
        Value string `json:"value"`
    }

    // Create handles POST /api/v1/projects/:pid/secrets.
    func (h *SecretsHandler) Create(c echo.Context) error {
        projectID := appMiddleware.GetProjectID(c)
        req := new(createSecretReq)
        if err := c.Bind(req); err != nil {
            return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
        }
        if req.Name == "" || req.Value == "" {
            return c.JSON(http.StatusBadRequest, map[string]string{"error": "name_and_value_required"})
        }
        actor := callerUserID(c)
        rec, err := h.svc.CreateSecret(c, projectID, req.Name, req.Value, req.Description, actor)
        if err != nil {
            if errors.Is(err, secrets.ErrNameConflict) {
                return c.JSON(http.StatusConflict, map[string]string{"error": "secret:name_conflict"})
            }
            return c.JSON(http.StatusInternalServerError, map[string]string{"error": "secret:create_failed"})
        }
        return c.JSON(http.StatusCreated, createSecretResp{
            secretMetadataDTO: toMetadataDTO(rec),
            Value:             req.Value,
        })
    }

    type rotateSecretReq struct {
        Value       *string `json:"value,omitempty"`
        Description *string `json:"description,omitempty"`
    }

    // Rotate handles PATCH /api/v1/projects/:pid/secrets/:name. Only `value`
    // is wired in 1B; `description` updates land in a follow-up PR (out of
    // scope for this slice but kept in the request shape so the FE can
    // already send it).
    func (h *SecretsHandler) Rotate(c echo.Context) error {
        projectID := appMiddleware.GetProjectID(c)
        name := c.Param("name")
        if name == "" {
            return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing_name"})
        }
        req := new(rotateSecretReq)
        if err := c.Bind(req); err != nil {
            return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
        }
        if req.Value == nil || *req.Value == "" {
            return c.JSON(http.StatusBadRequest, map[string]string{"error": "value_required"})
        }
        actor := callerUserID(c)
        if err := h.svc.RotateSecret(c, projectID, name, *req.Value, actor); err != nil {
            if errors.Is(err, secrets.ErrSecretNotFound) {
                return c.JSON(http.StatusNotFound, map[string]string{"error": "secret:not_found"})
            }
            return c.JSON(http.StatusInternalServerError, map[string]string{"error": "secret:rotate_failed"})
        }
        // Echo the new value once, same contract as Create.
        return c.JSON(http.StatusOK, map[string]any{
            "name":  name,
            "value": *req.Value,
        })
    }

    // Delete handles DELETE /api/v1/projects/:pid/secrets/:name.
    func (h *SecretsHandler) Delete(c echo.Context) error {
        projectID := appMiddleware.GetProjectID(c)
        name := c.Param("name")
        if name == "" {
            return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing_name"})
        }
        actor := callerUserID(c)
        if err := h.svc.DeleteSecret(c, projectID, name, actor); err != nil {
            if errors.Is(err, secrets.ErrSecretNotFound) {
                return c.JSON(http.StatusNotFound, map[string]string{"error": "secret:not_found"})
            }
            return c.JSON(http.StatusInternalServerError, map[string]string{"error": "secret:delete_failed"})
        }
        return c.NoContent(http.StatusNoContent)
    }

    // callerUserID extracts the actor uuid from JWT claims. Returns uuid.Nil
    // when claims are absent — Require() should have already rejected such
    // requests, so this is a defense-in-depth fallback.
    func callerUserID(c echo.Context) uuid.UUID {
        claims, err := appMiddleware.GetClaims(c)
        if err != nil || claims == nil {
            return uuid.Nil
        }
        id, err := uuid.Parse(claims.UserID)
        if err != nil {
            return uuid.Nil
        }
        return id
    }
    ```
  - Note: `Service` from Task 4 takes `context.Context`, not `echo.Context`. Add a thin wrapper struct in the same `handler/secrets_handler.go` file (or in `internal/secrets/echo_adapter.go`) that satisfies `secretsService` by translating `echo.Context` → `c.Request().Context()`:
    ```go
    // EchoServiceAdapter adapts the context.Context-based Service to the
    // echo.Context-based handler contract.
    type EchoServiceAdapter struct{ S *secrets.Service }

    func (a *EchoServiceAdapter) CreateSecret(c echo.Context, projectID uuid.UUID, name, plaintext, description string, actor uuid.UUID) (*secrets.Record, error) {
        return a.S.CreateSecret(c.Request().Context(), projectID, name, plaintext, description, actor)
    }
    func (a *EchoServiceAdapter) RotateSecret(c echo.Context, projectID uuid.UUID, name, plaintext string, actor uuid.UUID) error {
        return a.S.RotateSecret(c.Request().Context(), projectID, name, plaintext, actor)
    }
    func (a *EchoServiceAdapter) DeleteSecret(c echo.Context, projectID uuid.UUID, name string, actor uuid.UUID) error {
        return a.S.DeleteSecret(c.Request().Context(), projectID, name, actor)
    }
    func (a *EchoServiceAdapter) ListSecrets(c echo.Context, projectID uuid.UUID) ([]*secrets.Record, error) {
        return a.S.ListSecrets(c.Request().Context(), projectID)
    }
    ```
  - Run `rtk go test ./internal/handler/... -run Secrets` — passes.

- [ ] Step 7.3 — commit: `feat(secrets): HTTP CRUD with one-time value response and RBAC gating`

---

## Task 8 — Wire secrets subsystem into server bootstrap

- [ ] Step 8.1 — read env, build cipher, fail-fast on missing key
  - File: `src-go/internal/server/routes.go`
  - In the existing service-construction region (search for `auditSvc := service.NewAuditService(`), add immediately after the audit emitter is registered:
    ```go
    // Secrets subsystem. AGENTFORGE_SECRETS_KEY is REQUIRED — failing
    // closed here is the single source of truth for spec1 §11 master-key
    // handling.
    secretsKey := os.Getenv("AGENTFORGE_SECRETS_KEY")
    if secretsKey == "" {
        log.Fatal("AGENTFORGE_SECRETS_KEY is required (32 raw bytes or 44-char base64)")
    }
    secretsCipher, err := secrets.NewCipher(secretsKey)
    if err != nil {
        log.Fatalf("init secrets cipher: %v", err)
    }
    secretsRepo := secrets.NewGormRepo(taskRepo.DB())
    secretsAuditAdapter := secrets.NewAuditServiceAdapter(secretsAuditEmitter{svc: auditSvc})
    secretsSvc := secrets.NewService(secretsRepo, secretsCipher, secretsAuditAdapter)
    ```
  - Add a tiny shim type at the bottom of the file (or in a new `secrets_audit_emitter.go` next to the other adapter shims):
    ```go
    type secretsAuditEmitter struct{ svc *service.AuditService }

    func (e secretsAuditEmitter) Record(ctx context.Context, ev *model.AuditEvent) error {
        return e.svc.RecordEvent(ctx, ev)
    }
    ```
  - Add the route registration just below the existing employee handler block (around line 1037):
    ```go
    secretsH := handler.NewSecretsHandler(&handler.EchoServiceAdapter{S: secretsSvc})
    secretsH.Register(projectGroup)
    ```
  - Update imports at the top of the file to include `"os"`, `"github.com/react-go-quick-starter/server/internal/secrets"`, and (only if missing) `"github.com/react-go-quick-starter/server/internal/handler"`.

- [ ] Step 8.2 — verify wiring compiles + secrets routes are mounted
  - Run `rtk go build ./...` from `src-go/` — must succeed.
  - Add to `src-go/internal/server/routes_wiring_test.go` an assertion that `/api/v1/projects/:pid/secrets` (GET, POST), `/api/v1/projects/:pid/secrets/:name` (PATCH, DELETE) are registered. Pattern: copy the closest existing test (e.g. one that asserts employees routes mount).

- [ ] Step 8.3 — commit: `feat(secrets): wire cipher + repo + service + handler into server bootstrap`

---

## Task 9 — Integration test (real Postgres)

- [ ] Step 9.1 — write end-to-end integration test
  - File: `src-go/internal/secrets/repo_integration_test.go`
    ```go
    //go:build integration

    package secrets_test

    import (
        "context"
        "testing"

        "github.com/google/uuid"

        "github.com/react-go-quick-starter/server/internal/secrets"
        "github.com/react-go-quick-starter/server/internal/repository"
    )

    // TestMain in the repository package's user_repo_integration_test.go owns
    // the Postgres fixture; this test reuses repository.NewTestDB().

    func TestGormRepo_EndToEnd(t *testing.T) {
        db := repository.NewTestDB(t) // helper; reuse if it already exists
        ctx := context.Background()

        c, err := secrets.NewCipher(testKey)
        if err != nil {
            t.Fatalf("cipher: %v", err)
        }
        repo := secrets.NewGormRepo(db)
        svc := secrets.NewService(repo, c, nil)

        proj := mustSeedProject(t, db) // helper that inserts a row in projects + returns its uuid
        actor := uuid.New()

        rec, err := svc.CreateSecret(ctx, proj, "GITHUB_TOKEN", "ghp_xyz", "review", actor)
        if err != nil {
            t.Fatalf("create: %v", err)
        }
        if rec.ID == uuid.Nil {
            t.Fatal("expected ID to be populated")
        }

        // Conflict on second create.
        if _, err := svc.CreateSecret(ctx, proj, "GITHUB_TOKEN", "x", "", actor); err == nil {
            t.Fatal("expected name conflict")
        }

        plain, err := svc.Resolve(ctx, proj, "GITHUB_TOKEN")
        if err != nil || plain != "ghp_xyz" {
            t.Fatalf("resolve: %q err=%v", plain, err)
        }

        if err := svc.RotateSecret(ctx, proj, "GITHUB_TOKEN", "ghp_new", actor); err != nil {
            t.Fatalf("rotate: %v", err)
        }
        plain2, _ := svc.Resolve(ctx, proj, "GITHUB_TOKEN")
        if plain2 != "ghp_new" {
            t.Fatalf("rotate not reflected: got %q", plain2)
        }

        if err := svc.DeleteSecret(ctx, proj, "GITHUB_TOKEN", actor); err != nil {
            t.Fatalf("delete: %v", err)
        }
        if _, err := svc.Resolve(ctx, proj, "GITHUB_TOKEN"); err == nil {
            t.Fatal("expected not_found after delete")
        }
    }
    ```
  - If `repository.NewTestDB` and `mustSeedProject` helpers do not yet exist in this exact form, search `repository/*_integration_test.go` for the closest analog (e.g. `newIntegrationDB`, `seedProject`) and use that. The implementation does not need to invent new fixtures.

- [ ] Step 9.2 — verify
  - Run `rtk go test -tags=integration ./internal/secrets/...` from `src-go/`. Requires PG + the migration applied (see Task 1.4).

- [ ] Step 9.3 — commit: `test(secrets): integration coverage for end-to-end CRUD against Postgres`

---

## Task 10 — Frontend store `lib/stores/secrets-store.ts`

- [ ] Step 10.1 — write failing store test
  - File: `lib/stores/secrets-store.test.ts`
    ```ts
    jest.mock("@/lib/api-client", () => ({
      createApiClient: jest.fn(),
    }));

    jest.mock("./auth-store", () => ({
      useAuthStore: {
        getState: jest.fn(() => ({ accessToken: "test-token" })),
      },
    }));

    jest.mock("sonner", () => ({
      toast: { success: jest.fn(), error: jest.fn() },
    }));

    import { createApiClient } from "@/lib/api-client";
    import { useSecretsStore, type SecretMetadata } from "./secrets-store";

    const sample: SecretMetadata = {
      name: "GITHUB_TOKEN",
      description: "review token",
      createdBy: "user-1",
      createdAt: "2026-04-20T00:00:00Z",
      updatedAt: "2026-04-20T00:00:00Z",
    };

    describe("useSecretsStore", () => {
      beforeEach(() => {
        useSecretsStore.setState({ secretsByProject: {}, loadingByProject: {}, lastRevealedValue: null });
        jest.clearAllMocks();
      });

      it("fetches secrets metadata into the project slot", async () => {
        const api = { get: jest.fn(), post: jest.fn(), patch: jest.fn(), delete: jest.fn() };
        (createApiClient as jest.Mock).mockReturnValue(api);
        api.get.mockResolvedValue({ data: [sample] });

        await useSecretsStore.getState().fetchSecrets("proj-1");

        expect(api.get).toHaveBeenCalledWith(
          "/api/v1/projects/proj-1/secrets",
          { token: "test-token" },
        );
        expect(useSecretsStore.getState().secretsByProject["proj-1"]).toEqual([sample]);
      });

      it("captures the one-time value on create and clears it on consumeRevealedValue", async () => {
        const api = { get: jest.fn(), post: jest.fn(), patch: jest.fn(), delete: jest.fn() };
        (createApiClient as jest.Mock).mockReturnValue(api);
        api.post.mockResolvedValue({ data: { ...sample, value: "ghp_xyz" } });

        const created = await useSecretsStore
          .getState()
          .createSecret("proj-1", "GITHUB_TOKEN", "ghp_xyz", "review token");

        expect(created?.name).toBe("GITHUB_TOKEN");
        expect(useSecretsStore.getState().lastRevealedValue).toEqual({
          projectId: "proj-1",
          name: "GITHUB_TOKEN",
          value: "ghp_xyz",
        });

        useSecretsStore.getState().consumeRevealedValue();
        expect(useSecretsStore.getState().lastRevealedValue).toBeNull();
      });

      it("surfaces rotated value through lastRevealedValue", async () => {
        const api = { get: jest.fn(), post: jest.fn(), patch: jest.fn(), delete: jest.fn() };
        (createApiClient as jest.Mock).mockReturnValue(api);
        api.patch.mockResolvedValue({ data: { name: "GITHUB_TOKEN", value: "ghp_new" } });

        await useSecretsStore.getState().rotateSecret("proj-1", "GITHUB_TOKEN", "ghp_new");

        expect(useSecretsStore.getState().lastRevealedValue?.value).toBe("ghp_new");
      });

      it("removes from the project slot on delete", async () => {
        const api = { get: jest.fn(), post: jest.fn(), patch: jest.fn(), delete: jest.fn() };
        (createApiClient as jest.Mock).mockReturnValue(api);
        api.delete.mockResolvedValue({ data: null });
        useSecretsStore.setState({
          secretsByProject: { "proj-1": [sample] },
          loadingByProject: {},
          lastRevealedValue: null,
        });

        await useSecretsStore.getState().deleteSecret("proj-1", "GITHUB_TOKEN");

        expect(useSecretsStore.getState().secretsByProject["proj-1"]).toEqual([]);
      });
    });
    ```
  - Run `rtk vitest run lib/stores/secrets-store.test.ts` (or `rtk pnpm test -- lib/stores/secrets-store.test.ts` if Jest is the configured runner — repo currently uses Jest per `pnpm test`). Fails: store missing.

- [ ] Step 10.2 — implement the store
  - File: `lib/stores/secrets-store.ts`
    ```ts
    "use client";

    import { create } from "zustand";
    import { toast } from "sonner";
    import { createApiClient } from "@/lib/api-client";
    import { useAuthStore } from "./auth-store";

    const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

    export interface SecretMetadata {
      name: string;
      description?: string;
      lastUsedAt?: string;
      createdBy: string;
      createdAt: string;
      updatedAt: string;
    }

    export interface RevealedValue {
      projectId: string;
      name: string;
      value: string;
    }

    interface SecretsStoreState {
      secretsByProject: Record<string, SecretMetadata[]>;
      loadingByProject: Record<string, boolean>;
      // The last secret value the backend handed back on create or rotate.
      // Held in memory only; the FE clears it once the user dismisses the
      // reveal dialog.
      lastRevealedValue: RevealedValue | null;

      fetchSecrets: (projectId: string) => Promise<void>;
      createSecret: (
        projectId: string,
        name: string,
        value: string,
        description?: string,
      ) => Promise<SecretMetadata | null>;
      rotateSecret: (projectId: string, name: string, value: string) => Promise<void>;
      deleteSecret: (projectId: string, name: string) => Promise<void>;
      consumeRevealedValue: () => void;
    }

    const getApi = () => createApiClient(API_URL);
    const getToken = () => {
      const state = useAuthStore.getState() as { accessToken?: string | null; token?: string | null };
      return state.accessToken ?? state.token ?? null;
    };

    export const useSecretsStore = create<SecretsStoreState>()((set, get) => ({
      secretsByProject: {},
      loadingByProject: {},
      lastRevealedValue: null,

      fetchSecrets: async (projectId) => {
        const token = getToken();
        if (!token) return;
        set((s) => ({ loadingByProject: { ...s.loadingByProject, [projectId]: true } }));
        try {
          const { data } = await getApi().get<SecretMetadata[]>(
            `/api/v1/projects/${projectId}/secrets`,
            { token },
          );
          set((s) => ({
            secretsByProject: { ...s.secretsByProject, [projectId]: data ?? [] },
          }));
        } catch (err) {
          toast.error(`加载密钥失败: ${(err as Error).message}`);
        } finally {
          set((s) => ({ loadingByProject: { ...s.loadingByProject, [projectId]: false } }));
        }
      },

      createSecret: async (projectId, name, value, description) => {
        const token = getToken();
        if (!token) return null;
        try {
          const { data } = await getApi().post<SecretMetadata & { value: string }>(
            `/api/v1/projects/${projectId}/secrets`,
            { name, value, description: description ?? "" },
            { token },
          );
          const { value: revealed, ...metadata } = data;
          set((s) => ({
            secretsByProject: {
              ...s.secretsByProject,
              [projectId]: [metadata, ...(s.secretsByProject[projectId] ?? [])],
            },
            lastRevealedValue: { projectId, name, value: revealed },
          }));
          toast.success(`密钥 ${name} 已创建`);
          return metadata;
        } catch (err) {
          toast.error(`创建密钥失败: ${(err as Error).message}`);
          return null;
        }
      },

      rotateSecret: async (projectId, name, value) => {
        const token = getToken();
        if (!token) return;
        try {
          const { data } = await getApi().patch<{ name: string; value: string }>(
            `/api/v1/projects/${projectId}/secrets/${encodeURIComponent(name)}`,
            { value },
            { token },
          );
          set({ lastRevealedValue: { projectId, name, value: data.value } });
          toast.success(`密钥 ${name} 已轮换`);
          // Refresh the metadata list so updatedAt advances.
          await get().fetchSecrets(projectId);
        } catch (err) {
          toast.error(`轮换密钥失败: ${(err as Error).message}`);
        }
      },

      deleteSecret: async (projectId, name) => {
        const token = getToken();
        if (!token) return;
        try {
          await getApi().delete(
            `/api/v1/projects/${projectId}/secrets/${encodeURIComponent(name)}`,
            { token },
          );
          set((s) => ({
            secretsByProject: {
              ...s.secretsByProject,
              [projectId]: (s.secretsByProject[projectId] ?? []).filter((r) => r.name !== name),
            },
          }));
          toast.success("密钥已删除");
        } catch (err) {
          toast.error(`删除密钥失败: ${(err as Error).message}`);
        }
      },

      consumeRevealedValue: () => set({ lastRevealedValue: null }),
    }));
    ```
  - Run `rtk pnpm test -- lib/stores/secrets-store.test.ts` — passes.

- [ ] Step 10.3 — commit: `feat(fe): secrets-store with one-time-reveal capture`

---

## Task 11 — Frontend page `app/(dashboard)/projects/[id]/secrets/page.tsx`

- [ ] Step 11.1 — write the page
  - File: `app/(dashboard)/projects/[id]/secrets/page.tsx`
    ```tsx
    "use client";

    import { useEffect, useState } from "react";
    import { useParams } from "next/navigation";
    import { Copy, Plus, RotateCw, Trash2 } from "lucide-react";
    import { Button } from "@/components/ui/button";
    import { Input } from "@/components/ui/input";
    import { Label } from "@/components/ui/label";
    import { Skeleton } from "@/components/ui/skeleton";
    import {
      Dialog,
      DialogContent,
      DialogDescription,
      DialogFooter,
      DialogHeader,
      DialogTitle,
    } from "@/components/ui/dialog";
    import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
    import { PageHeader } from "@/components/shared/page-header";
    import { SectionCard } from "@/components/shared/section-card";
    import { EmptyState } from "@/components/shared/empty-state";
    import { useSecretsStore, type SecretMetadata } from "@/lib/stores/secrets-store";
    import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";
    import { toast } from "sonner";

    export default function ProjectSecretsPage() {
      const params = useParams<{ id: string }>();
      const projectId = params?.id ?? "";
      useBreadcrumbs([
        { label: "Projects", href: "/projects" },
        { label: projectId, href: `/projects/${projectId}` },
        { label: "Secrets" },
      ]);

      const {
        secretsByProject,
        loadingByProject,
        lastRevealedValue,
        fetchSecrets,
        createSecret,
        rotateSecret,
        deleteSecret,
        consumeRevealedValue,
      } = useSecretsStore();

      const [createOpen, setCreateOpen] = useState(false);
      const [rotateTarget, setRotateTarget] = useState<SecretMetadata | null>(null);

      useEffect(() => {
        if (projectId) fetchSecrets(projectId);
      }, [projectId, fetchSecrets]);

      const rows = secretsByProject[projectId] ?? [];
      const loading = loadingByProject[projectId] ?? false;

      return (
        <div className="flex flex-col gap-6">
          <PageHeader
            title="项目密钥"
            description="管理项目级敏感凭证。值仅在创建/轮换时一次性返回。"
            actions={
              <Button size="sm" onClick={() => setCreateOpen(true)}>
                <Plus className="mr-1 size-4" />
                新建密钥
              </Button>
            }
          />

          <SectionCard
            title="密钥列表"
            description="本表只显示元数据。明文值不会被存储或再次返回。"
          >
            {loading ? (
              <div className="flex flex-col gap-2">
                {Array.from({ length: 3 }).map((_, i) => (
                  <Skeleton key={i} className="h-10 w-full" />
                ))}
              </div>
            ) : rows.length === 0 ? (
              <EmptyState title="尚未创建任何密钥" />
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>名称</TableHead>
                    <TableHead>描述</TableHead>
                    <TableHead>最近使用</TableHead>
                    <TableHead>创建时间</TableHead>
                    <TableHead className="text-right">操作</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {rows.map((r) => (
                    <TableRow key={r.name}>
                      <TableCell className="font-mono">{r.name}</TableCell>
                      <TableCell className="text-muted-foreground">{r.description ?? "—"}</TableCell>
                      <TableCell>{r.lastUsedAt ? new Date(r.lastUsedAt).toLocaleString() : "—"}</TableCell>
                      <TableCell>{new Date(r.createdAt).toLocaleString()}</TableCell>
                      <TableCell className="flex justify-end gap-2">
                        <Button size="sm" variant="outline" onClick={() => setRotateTarget(r)}>
                          <RotateCw className="mr-1 size-3" />
                          轮换
                        </Button>
                        <Button
                          size="sm"
                          variant="destructive"
                          onClick={async () => {
                            if (confirm(`确认删除 ${r.name}？`)) {
                              await deleteSecret(projectId, r.name);
                            }
                          }}
                        >
                          <Trash2 className="mr-1 size-3" />
                          删除
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </SectionCard>

          <CreateSecretDialog
            open={createOpen}
            onClose={() => setCreateOpen(false)}
            onSubmit={async (name, value, description) => {
              await createSecret(projectId, name, value, description);
              setCreateOpen(false);
            }}
          />

          {rotateTarget && (
            <RotateSecretDialog
              target={rotateTarget}
              onClose={() => setRotateTarget(null)}
              onSubmit={async (value) => {
                await rotateSecret(projectId, rotateTarget.name, value);
                setRotateTarget(null);
              }}
            />
          )}

          {lastRevealedValue && lastRevealedValue.projectId === projectId && (
            <RevealedValueDialog
              name={lastRevealedValue.name}
              value={lastRevealedValue.value}
              onClose={consumeRevealedValue}
            />
          )}
        </div>
      );
    }

    function CreateSecretDialog({
      open,
      onClose,
      onSubmit,
    }: {
      open: boolean;
      onClose: () => void;
      onSubmit: (name: string, value: string, description: string) => Promise<void>;
    }) {
      const [name, setName] = useState("");
      const [value, setValue] = useState("");
      const [description, setDescription] = useState("");
      return (
        <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>新建密钥</DialogTitle>
              <DialogDescription>名称在项目内必须唯一；值仅在本次响应中显示一次。</DialogDescription>
            </DialogHeader>
            <form
              className="flex flex-col gap-3"
              onSubmit={async (e) => {
                e.preventDefault();
                if (!name || !value) return;
                await onSubmit(name, value, description);
                setName("");
                setValue("");
                setDescription("");
              }}
            >
              <div className="flex flex-col gap-1">
                <Label htmlFor="secret-name">名称</Label>
                <Input id="secret-name" value={name} onChange={(e) => setName(e.target.value)} required />
              </div>
              <div className="flex flex-col gap-1">
                <Label htmlFor="secret-value">值</Label>
                <Input
                  id="secret-value"
                  type="password"
                  value={value}
                  onChange={(e) => setValue(e.target.value)}
                  required
                />
              </div>
              <div className="flex flex-col gap-1">
                <Label htmlFor="secret-desc">描述（可选）</Label>
                <Input
                  id="secret-desc"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                />
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" onClick={onClose}>
                  取消
                </Button>
                <Button type="submit">创建</Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      );
    }

    function RotateSecretDialog({
      target,
      onClose,
      onSubmit,
    }: {
      target: SecretMetadata;
      onClose: () => void;
      onSubmit: (value: string) => Promise<void>;
    }) {
      const [value, setValue] = useState("");
      return (
        <Dialog open onOpenChange={(o) => !o && onClose()}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>轮换 {target.name}</DialogTitle>
              <DialogDescription>新值仅会显示一次；旧值立即失效。</DialogDescription>
            </DialogHeader>
            <form
              className="flex flex-col gap-3"
              onSubmit={async (e) => {
                e.preventDefault();
                if (!value) return;
                await onSubmit(value);
                setValue("");
              }}
            >
              <Label htmlFor="rotate-value">新值</Label>
              <Input
                id="rotate-value"
                type="password"
                value={value}
                onChange={(e) => setValue(e.target.value)}
                required
              />
              <DialogFooter>
                <Button type="button" variant="outline" onClick={onClose}>
                  取消
                </Button>
                <Button type="submit">轮换</Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      );
    }

    function RevealedValueDialog({
      name,
      value,
      onClose,
    }: {
      name: string;
      value: string;
      onClose: () => void;
    }) {
      return (
        <Dialog open onOpenChange={(o) => !o && onClose()}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>请保存 {name} 的值</DialogTitle>
              <DialogDescription className="text-destructive">
                关闭此对话框后将无法再次查看；请立即复制并妥善保管。
              </DialogDescription>
            </DialogHeader>
            <div className="flex items-center gap-2">
              <Input readOnly value={value} className="font-mono" />
              <Button
                size="sm"
                variant="outline"
                onClick={async () => {
                  await navigator.clipboard.writeText(value);
                  toast.success("已复制到剪贴板");
                }}
              >
                <Copy className="mr-1 size-3" />
                复制
              </Button>
            </div>
            <DialogFooter>
              <Button onClick={onClose}>我已保存</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      );
    }
    ```

- [ ] Step 11.2 — write smoke test for the page (one-time-reveal flow)
  - File: `app/(dashboard)/projects/[id]/secrets/page.test.tsx`
    ```tsx
    import { render, screen, fireEvent, waitFor } from "@testing-library/react";

    jest.mock("next/navigation", () => ({
      useParams: () => ({ id: "proj-1" }),
    }));

    jest.mock("@/hooks/use-breadcrumbs", () => ({ useBreadcrumbs: jest.fn() }));

    jest.mock("@/lib/stores/secrets-store", () => {
      const actual = jest.requireActual("@/lib/stores/secrets-store");
      return {
        ...actual,
        useSecretsStore: Object.assign(jest.fn(), { setState: jest.fn() }),
      };
    });

    import { useSecretsStore } from "@/lib/stores/secrets-store";
    import ProjectSecretsPage from "./page";

    describe("ProjectSecretsPage", () => {
      beforeEach(() => {
        (useSecretsStore as unknown as jest.Mock).mockReturnValue({
          secretsByProject: {
            "proj-1": [
              {
                name: "GITHUB_TOKEN",
                description: "review token",
                createdBy: "user-1",
                createdAt: "2026-04-20T00:00:00Z",
                updatedAt: "2026-04-20T00:00:00Z",
              },
            ],
          },
          loadingByProject: { "proj-1": false },
          lastRevealedValue: { projectId: "proj-1", name: "NEW_KEY", value: "ghp_xyz" },
          fetchSecrets: jest.fn(),
          createSecret: jest.fn(),
          rotateSecret: jest.fn(),
          deleteSecret: jest.fn(),
          consumeRevealedValue: jest.fn(),
        });
      });

      it("renders the secrets table and the one-time-reveal dialog", async () => {
        render(<ProjectSecretsPage />);

        expect(screen.getByText("GITHUB_TOKEN")).toBeInTheDocument();
        expect(screen.getByText(/请保存.*NEW_KEY.*的值/)).toBeInTheDocument();
        expect(screen.getByDisplayValue("ghp_xyz")).toBeInTheDocument();
      });

      it("clears the revealed value on dismiss", async () => {
        const consume = jest.fn();
        (useSecretsStore as unknown as jest.Mock).mockReturnValue({
          secretsByProject: { "proj-1": [] },
          loadingByProject: {},
          lastRevealedValue: { projectId: "proj-1", name: "X", value: "v" },
          fetchSecrets: jest.fn(),
          createSecret: jest.fn(),
          rotateSecret: jest.fn(),
          deleteSecret: jest.fn(),
          consumeRevealedValue: consume,
        });
        render(<ProjectSecretsPage />);
        fireEvent.click(screen.getByText("我已保存"));
        await waitFor(() => expect(consume).toHaveBeenCalled());
      });
    });
    ```
  - Run `rtk pnpm test -- app/(dashboard)/projects/[id]/secrets` — passes.

- [ ] Step 11.3 — note nav-wiring follow-up
  - This slice does NOT add a sidebar entry because `app/(dashboard)/projects/[id]/layout.tsx` does not yet exist (Glob confirmed). Add a TODO comment at the top of `page.tsx`:
    ```tsx
    // TODO(spec1-1A): once projects/[id]/layout.tsx ships, add a "Secrets"
    // entry to the sidebar that links here.
    ```

- [ ] Step 11.4 — commit: `feat(fe): project secrets page with create/rotate/delete + one-time reveal`

---

## Task 12 — Self-review + spec coverage check

- [ ] Step 12.1 — confirm spec §6.1 / §7 / §11 coverage
  - Re-read `docs/superpowers/specs/2026-04-20-foundation-gaps-design.md` §6.1, §7, §11.
  - Tick:
    - [x] secrets table matches §6.1 column-by-column (`id`, `project_id`, `name`, `ciphertext`, `nonce`, `key_version`, `description`, `last_used_at`, `created_by`, `created_at`, `updated_at`, `UNIQUE (project_id, name)`)
    - [x] §7 endpoints: GET (no values), POST (value once), PATCH (value once), DELETE — all four wired with project RBAC `secret.read|write`
    - [x] §11: AES-256-GCM, plaintext NEVER in dataStore/log/err/audit (test in 4.1 captures logs); `{{secrets.X}}` whitelist enforced both at save AND execution time; `AGENTFORGE_SECRETS_KEY` fail-fast; CRUD audited via `RecordEvent`
    - [x] §14 last bullet: `{{system_metadata.*}}` rejected from author-controlled templates (resolver test in 6.1)

- [ ] Step 12.2 — run the full Go test suite
  - Run `rtk go test ./...` from `src-go/` — must be green (excluding `-tags=integration` which needs PG).
  - Run `rtk go test -tags=integration ./internal/secrets/...` if PG is available locally.

- [ ] Step 12.3 — run the FE test slice
  - Run `rtk pnpm test -- secrets-store secrets/page` — green.

- [ ] Step 12.4 — final check: nothing in this plan accidentally:
  - logs plaintext (search the diff for any `log.*plain` / `log.*value` reference in the secrets package)
  - serializes `Ciphertext` / `Nonce` in any DTO returned from the handler (confirm `secretMetadataDTO` has neither field)
  - touches `system_metadata` from author code
  - bypasses `Require(ActionSecretWrite)` on any mutating route
  - Note any deviation under `## 14 Open Risks` of the spec instead of silently fixing.

- [ ] Step 12.5 — commit: `chore(secrets): self-review pass; ready for 1E HTTP-node integration`
