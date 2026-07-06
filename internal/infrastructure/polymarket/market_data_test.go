package polymarket

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchTrades(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/trades" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("market"); got != "0xcond" {
			t.Errorf("unexpected market param: %s", got)
		}
		if got := r.URL.Query().Get("limit"); got != "2" {
			t.Errorf("unexpected limit param: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"proxyWallet":"0xaaa","side":"BUY","price":0.48,"size":100000,"conditionId":"0xcond","timestamp":1751600000},
			{"proxyWallet":"0xbbb","side":"SELL","price":0.47,"size":10000,"conditionId":"0xcond","timestamp":1751600100}
		]`))
	}))
	defer srv.Close()

	c := NewClientWithBaseURLs(slog.Default(), srv.URL, srv.URL, srv.URL)
	trades, err := c.FetchTrades(context.Background(), "0xcond", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trades) != 2 {
		t.Fatalf("expected 2 trades, got %d", len(trades))
	}
	if trades[0].Wallet != "0xaaa" || trades[0].Side != "BUY" || trades[0].Price != 0.48 || trades[0].Size != 100000 {
		t.Errorf("unexpected first trade: %+v", trades[0])
	}
}

func TestFetchTradesUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal","message":"boom"}`))
	}))
	defer srv.Close()

	c := NewClientWithBaseURLs(slog.Default(), srv.URL, srv.URL, srv.URL)
	if _, err := c.FetchTrades(context.Background(), "0xcond", 10); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchOrderbook(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/book" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("token_id"); got != "tok123" {
			t.Errorf("unexpected token_id param: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"asset_id":"tok123",
			"bids":[{"price":"0.54","size":"150"}],
			"asks":[{"price":"0.55","size":"100"}]
		}`))
	}))
	defer srv.Close()

	c := NewClientWithBaseURLs(slog.Default(), srv.URL, srv.URL, srv.URL)
	ob, err := c.FetchOrderbook(context.Background(), "tok123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ob.AssetID != "tok123" || len(ob.Bids) != 1 || len(ob.Asks) != 1 {
		t.Errorf("unexpected orderbook: %+v", ob)
	}
	if ob.Bids[0].Price != "0.54" || ob.Asks[0].Size != "100" {
		t.Errorf("unexpected levels: %+v", ob)
	}
}
