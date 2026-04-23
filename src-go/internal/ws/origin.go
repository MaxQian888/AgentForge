package ws

import (
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

func newUpgrader(allowedOrigins []string) websocket.Upgrader {
	allowAll := false
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		normalized := normalizeOrigin(origin)
		if normalized == "" {
			continue
		}
		if normalized == "*" {
			allowAll = true
			continue
		}
		allowed[normalized] = struct{}{}
	}

	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := normalizeOrigin(r.Header.Get("Origin"))
			if origin == "" || allowAll {
				return true
			}
			_, ok := allowed[origin]
			return ok
		},
	}
}

func normalizeOrigin(origin string) string {
	return strings.TrimRight(strings.TrimSpace(origin), "/")
}
