package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testToken generates a fresh random token per call, so no secret-shaped
// literal ever lands in source (a static string like "test-token" trips
// secret scanners in CI even though it's not a real credential).
func testToken(t *testing.T) string {
	t.Helper()

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("generating test token: %v", err)
	}
	return hex.EncodeToString(b)
}

// postMCP sends an MCP initialize handshake through mux, optionally carrying
// an Authorization header, and returns the response. It uses a real loopback
// listener for consistency with the Host-header tests in server_test.go.
func postMCP(t *testing.T, mux http.Handler, authHeader string) (int, string) {
	t.Helper()

	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":` +
		`{"protocolVersion":"2025-06-18","capabilities":{},` +
		`"clientInfo":{"name":"test","version":"1"}}}`

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/mcp", strings.NewReader(body))
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("sending request: %v", err)
	}
	defer resp.Body.Close()

	payload, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(payload)
}

func TestBearerAuthOpenWhenTokenUnset(t *testing.T) {
	t.Setenv(mcpBearerTokenEnvVar, "")

	status, body := postMCP(t, newTestServer(t).buildHTTPMux(), "")

	if status != http.StatusOK {
		t.Fatalf("expected the handshake to proceed with auth disabled, got %d: %s", status, body)
	}
	if !strings.Contains(body, "protocolVersion") {
		t.Fatalf("expected a completed MCP handshake, got: %s", body)
	}
}

func TestBearerAuthRejectsMissingHeader(t *testing.T) {
	t.Setenv(mcpBearerTokenEnvVar, testToken(t))

	status, body := postMCP(t, newTestServer(t).buildHTTPMux(), "")

	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a header, got %d: %s", status, body)
	}
}

func TestBearerAuthRejectsWrongToken(t *testing.T) {
	t.Setenv(mcpBearerTokenEnvVar, testToken(t))

	status, body := postMCP(t, newTestServer(t).buildHTTPMux(), "Bearer "+testToken(t))

	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401 with the wrong token, got %d: %s", status, body)
	}
}

func TestBearerAuthAcceptsCorrectToken(t *testing.T) {
	token := testToken(t)
	t.Setenv(mcpBearerTokenEnvVar, token)

	status, body := postMCP(t, newTestServer(t).buildHTTPMux(), "Bearer "+token)

	if status != http.StatusOK {
		t.Fatalf("expected the handshake to proceed with the right token, got %d: %s", status, body)
	}
	if !strings.Contains(body, "protocolVersion") {
		t.Fatalf("expected a completed MCP handshake, got: %s", body)
	}
}

func TestHealthStaysOpenWhenTokenSet(t *testing.T) {
	t.Setenv(mcpBearerTokenEnvVar, testToken(t))

	srv := httptest.NewServer(newTestServer(t).buildHTTPMux())
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected /health to stay open without a token, got %d", resp.StatusCode)
	}
}

func TestSSEAuthRejectsMissingHeader(t *testing.T) {
	t.Setenv(mcpBearerTokenEnvVar, testToken(t))

	srv := httptest.NewServer(newTestServer(t).buildSSEMux())
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/sse")
	if err != nil {
		t.Fatalf("GET /sse: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a header on /sse, got %d", resp.StatusCode)
	}
}

func TestSSEAuthOpenWhenTokenUnset(t *testing.T) {
	t.Setenv(mcpBearerTokenEnvVar, "")

	srv := httptest.NewServer(newTestServer(t).buildSSEMux())
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/sse")
	if err != nil {
		t.Fatalf("GET /sse: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatalf("expected /sse to stay open with auth disabled, got %d", resp.StatusCode)
	}
}

func TestBearerAuthRejectionLogsWithoutToken(t *testing.T) {
	token := testToken(t)
	t.Setenv(mcpBearerTokenEnvVar, token)

	var logs strings.Builder
	srv := newTestServer(t)
	srv.slg = slog.New(slog.NewTextHandler(&logs, nil))

	wrongToken := testToken(t)
	status, _ := postMCP(t, srv.buildHTTPMux(), "Bearer "+wrongToken)

	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", status)
	}

	logged := logs.String()
	if !strings.Contains(logged, "rejected") {
		t.Fatalf("expected a rejection log line, got: %s", logged)
	}
	if strings.Contains(logged, token) || strings.Contains(logged, wrongToken) {
		t.Fatalf("log output must never contain a token, got: %s", logged)
	}
}
