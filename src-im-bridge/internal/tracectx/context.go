// Package tracectx exposes context-scoped correlation helpers for im-bridge.
package tracectx

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"strings"
)

type ctxKey struct{}

var key = ctxKey{}

// TraceID returns the trace_id attached to ctx, or "" if none.
func TraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(key).(string); ok {
		return v
	}
	return ""
}

// With returns a copy of ctx carrying the given trace_id.
func With(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, key, id)
}

var crockford = base32.NewEncoding("0123456789ABCDEFGHJKMNPQRSTVWXYZ").WithPadding(base32.NoPadding)

// New returns a fresh, URL-safe trace identifier "tr_" + 24-char lowercase crockford-base32 (15 random bytes).
func New() string {
	buf := make([]byte, 15)
	if _, err := rand.Read(buf); err != nil {
		panic("trace id: " + err.Error())
	}
	return "tr_" + strings.ToLower(crockford.EncodeToString(buf))
}
