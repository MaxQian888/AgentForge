package notify

import (
	"net/http"

	"github.com/agentforge/im-bridge/internal/tracectx"
)

// withTrace extracts X-Trace-ID from the inbound request (or generates a fresh
// one) and attaches it to the request context. The resolved id is echoed back
// on the response so callers can correlate their own traces.
func withTrace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Trace-ID")
		if id == "" {
			id = tracectx.New()
		}
		ctx := tracectx.With(r.Context(), id)
		w.Header().Set("X-Trace-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
