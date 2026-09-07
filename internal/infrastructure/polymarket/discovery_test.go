package polymarket

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The query must ask Gamma for open events in one tag. closed=false and
// archived=false are both load-bearing: without them Gamma returns settled
// events, and a "currently moving" feed would lead with markets whose price
// can never move again.
func TestFetchEventsByTagQueriesOpenEventsOnly(t *testing.T) {
	var got *http.Request

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewClientWithBaseURLs(slog.Default(), srv.URL, srv.URL, srv.URL)
	if _, err := c.FetchEventsByTag(context.Background(), "politics", 25); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.URL.Path != "/events" {
		t.Errorf("path = %q, want /events", got.URL.Path)
	}

	q := got.URL.Query()
	checks := map[string]string{
		"tag_slug": "politics",
		"closed":   "false",
		"archived": "false",
		"limit":    "25",
	}
	for key, want := range checks {
		if q.Get(key) != want {
			t.Errorf("query %s = %q, want %q (full: %s)", key, q.Get(key), want, got.URL.RawQuery)
		}
	}
}

// A tag slug is user-supplied by the time it reaches here, so it must be
// escaped rather than concatenated into the query string.
func TestFetchEventsByTagEscapesTheSlug(t *testing.T) {
	var got *http.Request

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewClientWithBaseURLs(slog.Default(), srv.URL, srv.URL, srv.URL)
	if _, err := c.FetchEventsByTag(context.Background(), "a b&c=d", 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if slug := got.URL.Query().Get("tag_slug"); slug != "a b&c=d" {
		t.Errorf("tag_slug round-tripped as %q, want %q", slug, "a b&c=d")
	}
}

func TestFetchEventsByTagDecodesEvents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":"1","slug":"us-recession-by-end-of-2026","title":"US recession","volume24hr":4200.5},
			{"id":"2","slug":"macron-out-in-2025","title":"Macron out","volume24hr":19.25}
		]`))
	}))
	defer srv.Close()

	c := NewClientWithBaseURLs(slog.Default(), srv.URL, srv.URL, srv.URL)
	events, err := c.FetchEventsByTag(context.Background(), "politics", 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if events[0].Slug != "us-recession-by-end-of-2026" {
		t.Errorf("first slug = %q", events[0].Slug)
	}
	// Gamma numeric fields that look integral arrive fractional; one bad type
	// fails the whole decode.
	if events[1].Volume24Hr != 19.25 {
		t.Errorf("volume24hr = %v, want 19.25", events[1].Volume24Hr)
	}
}

func TestFetchEventsByTagUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer srv.Close()

	c := NewClientWithBaseURLs(slog.Default(), srv.URL, srv.URL, srv.URL)
	if _, err := c.FetchEventsByTag(context.Background(), "politics", 25); err == nil {
		t.Fatal("expected error, got nil")
	}
}
