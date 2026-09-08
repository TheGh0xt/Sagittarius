package polymarket

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// B.2: TTL cache. Repeated GETs to the same URL within the TTL must not
// reach the upstream API at all -- this is what stops a default
// get_moving_markets call (14 tag fetches, and any repeat of it) from paying
// for the same request over and over.
func TestGETResponsesAreCachedWithinTTL(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cached-event","title":"first response"}`))
	}))
	defer srv.Close()

	c := NewClientWithBaseURLs(slog.Default(), srv.URL, srv.URL, srv.URL)

	for i := 0; i < 3; i++ {
		ev, err := c.FetchEventByID(context.Background(), "cached-event")
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
		if ev.ID != "cached-event" {
			t.Fatalf("call %d: unexpected event: %+v", i, ev)
		}
	}

	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("server saw %d requests, want 1 — the second and third calls should have hit the cache", got)
	}
}

// A cache entry outliving its TTL would silently serve stale prices forever.
func TestCacheEntryExpiresAfterTTL(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer srv.Close()

	c := NewClientWithBaseURLs(slog.Default(), srv.URL, srv.URL, srv.URL)
	c.cache = newTTLCache(10 * time.Millisecond)

	if _, err := c.FetchEventByID(context.Background(), "1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	time.Sleep(25 * time.Millisecond)
	if _, err := c.FetchEventByID(context.Background(), "1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Errorf("server saw %d requests, want 2 — the cache entry should have expired between calls", got)
	}
}

// Different URLs (here: different event IDs) must never collide in the
// cache -- each endpoint/query pair is cached independently.
func TestCacheKeysAreDistinctPerURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/events/1" {
			_, _ = w.Write([]byte(`{"id":"1","title":"first"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"2","title":"second"}`))
	}))
	defer srv.Close()

	c := NewClientWithBaseURLs(slog.Default(), srv.URL, srv.URL, srv.URL)

	ev1, err := c.FetchEventByID(context.Background(), "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ev2, err := c.FetchEventByID(context.Background(), "2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ev1.Title != "first" || ev2.Title != "second" {
		t.Errorf("cache collision: event 1 = %+v, event 2 = %+v", ev1, ev2)
	}
}

// B.2: outbound rate limiting. A request beyond the limiter's burst must
// wait rather than firing immediately, so a caller whose context expires
// first never reaches the upstream API at all.
func TestRateLimiterThrottlesOutboundRequests(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ok"}`))
	}))
	defer srv.Close()

	c := NewClientWithBaseURLs(slog.Default(), srv.URL, srv.URL, srv.URL)
	// One token, refilling once a second: the first call spends the burst,
	// the second must wait ~1s for a new one.
	c.limiter = rate.NewLimiter(rate.Limit(1), 1)

	if _, err := c.FetchEventByID(context.Background(), "1"); err != nil {
		t.Fatalf("first call (within burst) should succeed: %v", err)
	}

	shortCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := c.FetchEventByID(shortCtx, "2"); err == nil {
		t.Fatal("second call should have been throttled past the context deadline, got no error")
	}

	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("server saw %d requests, want 1 — the throttled call must never reach the upstream API", got)
	}
}

// B.2: retry/backoff. A transient (5xx) failure must be retried rather than
// failing the whole tool call.
func TestRetriesTransientUpstreamErrors(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Fail exactly twice, then succeed. This is independent of the
		// maxAttempts constant's exact value on purpose: it fails if maxAttempts
		// is ever reduced to 1 (no retry at all) or 2 (one retry, still not
		// enough to reach this fixture's third, successful response).
		n := atomic.AddInt32(&hits, 1)
		if n <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"unavailable","message":"try again"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer srv.Close()

	c := NewClientWithBaseURLs(slog.Default(), srv.URL, srv.URL, srv.URL)
	ev, err := c.FetchEventByID(context.Background(), "1")
	if err != nil {
		t.Fatalf("expected the third attempt to succeed, got: %v", err)
	}
	if ev.ID != "1" {
		t.Errorf("unexpected event: %+v", ev)
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Errorf("server saw %d requests, want 3 (2 failures + 1 success)", got)
	}
}

// Retries are not unbounded: once maxAttempts is spent against a
// permanently-failing upstream, the call must fail rather than retry forever.
func TestGivesUpAfterMaxAttempts(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"unavailable","message":"still down"}`))
	}))
	defer srv.Close()

	c := NewClientWithBaseURLs(slog.Default(), srv.URL, srv.URL, srv.URL)
	if _, err := c.FetchEventByID(context.Background(), "1"); err == nil {
		t.Fatal("expected an error once retries are exhausted")
	}
	if got := atomic.LoadInt32(&hits); got != int32(maxAttempts) {
		t.Errorf("server saw %d requests, want exactly %d (maxAttempts)", got, maxAttempts)
	}
}

// A 4xx means the request itself was rejected. Retrying it three times
// slower is pure waste and, for something like a malformed slug, cannot ever
// succeed.
func TestDoesNotRetryClientErrors(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not_found","message":"no such event"}`))
	}))
	defer srv.Close()

	c := NewClientWithBaseURLs(slog.Default(), srv.URL, srv.URL, srv.URL)
	if _, err := c.FetchEventByID(context.Background(), "1"); err == nil {
		t.Fatal("expected an error")
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("server saw %d requests, want 1 — a 404 must not be retried", got)
	}
}
