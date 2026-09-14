package mcp

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
	t.Setenv(mcpBearerTokenEnvVar, "s3cret")

	status, body := postMCP(t, newTestServer(t).buildHTTPMux(), "")

	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a header, got %d: %s", status, body)
	}
}

func TestBearerAuthRejectsWrongToken(t *testing.T) {
	t.Setenv(mcpBearerTokenEnvVar, "s3cret")

	status, body := postMCP(t, newTestServer(t).buildHTTPMux(), "Bearer wrong-token")

	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401 with the wrong token, got %d: %s", status, body)
	}
}

func TestBearerAuthAcceptsCorrectToken(t *testing.T) {
	t.Setenv(mcpBearerTokenEnvVar, "s3cret")

	status, body := postMCP(t, newTestServer(t).buildHTTPMux(), "Bearer s3cret")

	if status != http.StatusOK {
		t.Fatalf("expected the handshake to proceed with the right token, got %d: %s", status, body)
	}
	if !strings.Contains(body, "protocolVersion") {
		t.Fatalf("expected a completed MCP handshake, got: %s", body)
	}
}

func TestHealthStaysOpenWhenTokenSet(t *testing.T) {
	t.Setenv(mcpBearerTokenEnvVar, "s3cret")

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

func TestBearerAuthRejectionNeverLogsToken(t *testing.T) {
	t.Setenv(mcpBearerTokenEnvVar, "s3cret-do-not-log")

	var logs strings.Builder
	srv := newTestServer(t)
	srv.slg = slog.New(slog.NewTextHandler(&logs, nil))

	status, _ := postMCP(t, srv.buildHTTPMux(), "Bearer wrong-guess")

	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", status)
	}
	if strings.Contains(logs.String(), "s3cret-do-not-log") || strings.Contains(logs.String(), "wrong-guess") {
		t.Fatalf("log output must never contain a token, got: %s", logs.String())
	}
}
