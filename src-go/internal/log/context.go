// Package log exposes context-scoped correlation helpers for structured logging.
package log

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"strings"
)

type ctxKey struct{}

var traceIDKey = ctxKey{}

// TraceID returns the trace_id attached to ctx, or "" if none.
func TraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(traceIDKey).(string); ok {
		return v
	}
	return ""
}

// WithTrace returns a copy of ctx carrying the given trace_id.
func WithTrace(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceIDKey, id)
}

// crockford is Douglas Crockford's base32 alphabet (no I, L, O, U).
var crockford = base32.NewEncoding("0123456789ABCDEFGHJKMNPQRSTVWXYZ").WithPadding(base32.NoPadding)

// NewTraceID returns a fresh, URL-safe trace identifier "tr_" + 24-char crockford-base32 (15 random bytes).
func NewTraceID() string {
	buf := make([]byte, 15) // 15 bytes → 24 base32 chars
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand.Read never returns err on supported platforms; fail loud if it does.
		panic("trace id: " + err.Error())
	}
	return "tr_" + strings.ToLower(crockford.EncodeToString(buf))
}
