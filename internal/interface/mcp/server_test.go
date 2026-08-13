package mcp

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The SDK refuses requests whose Host header is not a loopback address when
// the connection arrives on a loopback local address, as DNS rebinding
// protection. Behind a platform proxy every request carries the public
// hostname, so without an opt-out the MCP endpoint answers
//
//	403 Forbidden: invalid Host header "<host>"
//
// to every real client while /health keeps working — the service looks
// deployed and is completely unusable.
//
// These use a real listener rather than httptest.NewRequest on purpose. The
// SDK only applies the check when the connection's local address is loopback,
// and a synthetic request has no connection at all, so the protection never
// fires and the test would pass against a broken build.

func newTestServer(t *testing.T) *Server {
	t.Helper()
	ms := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	return NewServer(ms, nil, nil, slog.New(slog.DiscardHandler))
}

// postInitialize sends an MCP handshake to a real loopback listener while
// presenting the given Host header, mimicking a proxied request.
func postInitialize(t *testing.T, host string) (int, string) {
	t.Helper()

	srv := httptest.NewServer(newTestServer(t).GetHTTPHandler(slog.New(slog.DiscardHandler)))
	defer srv.Close()

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":` +
		`{"protocolVersion":"2025-06-18","capabilities":{},` +
		`"clientInfo":{"name":"test","version":"1"}}}`

	req, err := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(body))
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	// Host is set on the struct, not the header map: net/http reads it from
	// here when writing the request line.
	req.Host = host
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("sending request: %v", err)
	}
	defer resp.Body.Close()

	payload, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(payload)
}

func TestBehindProxyDefaultsOff(t *testing.T) {
	t.Setenv("MCP_BEHIND_PROXY", "")
	if behindProxy() {
		t.Fatal("localhost protection must stay on unless explicitly opted out")
	}
}

func TestBehindProxyOnlyEnabledByExactValue(t *testing.T) {
	// A typo must not silently switch off a security default.
	for _, value := range []string{"", "0", "true", "yes", "TRUE"} {
		t.Setenv("MCP_BEHIND_PROXY", value)
		if behindProxy() {
			t.Fatalf("MCP_BEHIND_PROXY=%q should not disable protection", value)
		}
	}
	t.Setenv("MCP_BEHIND_PROXY", "1")
	if !behindProxy() {
		t.Fatal("MCP_BEHIND_PROXY=1 should disable protection")
	}
}

func TestNonLoopbackHostRejectedByDefault(t *testing.T) {
	os.Unsetenv("MCP_BEHIND_PROXY")

	status, body := postInitialize(t, "sagittarius-rp3z.onrender.com")

	if status != http.StatusForbidden {
		t.Fatalf("expected 403 without the opt-out, got %d: %s", status, body)
	}
	if !strings.Contains(body, "invalid Host header") {
		t.Fatalf("expected the SDK's host rejection, got: %s", body)
	}
}

func TestNonLoopbackHostAcceptedBehindProxy(t *testing.T) {
	t.Setenv("MCP_BEHIND_PROXY", "1")

	status, body := postInitialize(t, "sagittarius-rp3z.onrender.com")

	if status == http.StatusForbidden {
		t.Fatalf("public hostname still rejected behind a proxy: %s", body)
	}
	if !strings.Contains(body, "protocolVersion") {
		t.Fatalf("expected a completed MCP handshake, got: %s", body)
	}
}

func TestLoopbackHostAlwaysWorks(t *testing.T) {
	// Local development must keep working with protection enabled.
	os.Unsetenv("MCP_BEHIND_PROXY")

	status, body := postInitialize(t, "localhost:8080")

	if status == http.StatusForbidden {
		t.Fatalf("loopback host should never be rejected: %s", body)
	}
}
