package mcp

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

// mcpBearerTokenEnvVar, when set, requires every request to /mcp (and the SSE
// transport's /sse, /message) to carry a matching `Authorization: Bearer
// <token>` header. Left unset, the server behaves exactly as before B.3 and
// logs one warning at startup that auth is disabled.
//
// Rollout order matters: Cygnus (lane CR) ships the client side that sends
// this header first. Enforcement here begins only once an operator sets the
// same value in MCP_BEARER_TOKEN on both the Sagittarius and Cygnus Render
// services, after CR's change is deployed (WAVE1_LEDGER.md H4).
const mcpBearerTokenEnvVar = "MCP_BEARER_TOKEN"

// bearerAuthMiddleware reads mcpBearerTokenEnvVar once, at the point the
// caller builds the handler (server startup, or per test case) — not per
// request. An empty value disables auth entirely.
func bearerAuthMiddleware(next http.Handler, slg *slog.Logger) http.Handler {
	token := os.Getenv(mcpBearerTokenEnvVar)
	if token == "" {
		slg.Warn("Sagittarius", "state", "auth disabled",
			"detail", mcpBearerTokenEnvVar+" is unset; /mcp accepts unauthenticated requests")
		return next
	}

	expected := []byte(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := bearerToken(r.Header.Get("Authorization"))
		reason := "mismatch"
		if !ok {
			reason = "missing"
		}
		if !ok || subtle.ConstantTimeCompare([]byte(got), expected) != 1 {
			// During the H4 rollout, a client not yet sending the header is
			// otherwise silent here and only visible as an error on the
			// client's side. Never log the header or token value itself.
			slg.Info("Sagittarius", "state", "rejected unauthenticated request",
				"path", r.URL.Path, "remote_addr", r.RemoteAddr, "reason", reason)
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// bearerToken extracts the token from an `Authorization: Bearer <token>`
// header value. Neither the header nor the extracted token may be logged by
// any caller.
func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	return header[len(prefix):], true
}
