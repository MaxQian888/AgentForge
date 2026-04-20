// Package template extends the workflow template engine with two
// secrecy-sensitive primitives:
//
//  1. Substituting `{{secrets.NAME}}` references — but ONLY inside the
//     strict whitelist of HTTP-node config fields documented in
//     docs/superpowers/specs/2026-04-20-foundation-gaps-design.md §11.
//  2. Rejecting `{{system_metadata.*}}` references inside dataStore
//     template expressions, per spec §14 last bullet — author code
//     cannot read system_metadata.
//
// The package intentionally has no transitive imports of crypto or repo
// code: all secret access is delegated through the SecretSource interface
// so the resolver can be unit-tested without DB or cipher.
//
// API contract for HTTP node (Plan 1E): at execution time, for every header
// value, every url-query value, and the request body, call
// `resolver.Render(ctx, projectID, FieldHTTP{Headers,URLQuery,Body}, raw,
// dataStore)`. For every other config field, call
// `resolver.Render(ctx, projectID, FieldGeneric, raw, dataStore)`. At save
// time, call `ValidateNoSecretReferences(field, raw)` once per field;
// reject the workflow save with HTTP 400 if any field returns
// `ErrSecretFieldNotAllowed`.
package template

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/google/uuid"

	"github.com/agentforge/server/internal/workflow/nodetypes"
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
//  1. Reject disallowed `{{system_metadata.*}}` references first
//     (defense in depth — same rule applies to every field kind).
//  2. Reject `{{secrets.X}}` if field is not in the allowlist.
//  3. Substitute `{{secrets.X}}` via SecretSource.
//  4. Hand the remaining template string to nodetypes.ResolveTemplateVars
//     so existing `{{node.path}}` references continue to work.
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

// String returns the FieldKind as a string. Useful for callers that log
// or audit field kinds.
func (f FieldKind) String() string { return string(f) }
