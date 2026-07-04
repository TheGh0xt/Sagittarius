package polymarket

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchEventByID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/events/903193" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"903193","title":"Fed decision in October","volume":123.5}`))
	}))
	defer srv.Close()

	c := NewClientWithBaseURLs(slog.Default(), srv.URL, srv.URL, srv.URL)
	ev, err := c.FetchEventByID(context.Background(), "903193")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.ID != "903193" || ev.Title != "Fed decision in October" || ev.Volume != 123.5 {
		t.Errorf("unexpected event: %+v", ev)
	}
}

func TestFetchEventByIDUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found","message":"no event"}`))
	}))
	defer srv.Close()

	c := NewClientWithBaseURLs(slog.Default(), srv.URL, srv.URL, srv.URL)
	if _, err := c.FetchEventByID(context.Background(), "0"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchEventBySlugPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/events/slug/will-btc-hit-150k" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","slug":"will-btc-hit-150k","title":"Will BTC hit 150k?"}`))
	}))
	defer srv.Close()

	c := NewClientWithBaseURLs(slog.Default(), srv.URL, srv.URL, srv.URL)
	ev, err := c.FetchEventBySlug(context.Background(), "will-btc-hit-150k")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Slug != "will-btc-hit-150k" {
		t.Errorf("unexpected event: %+v", ev)
	}
}
