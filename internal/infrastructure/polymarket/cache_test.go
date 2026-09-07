package polymarket

import (
	"testing"
	"time"
)

func TestTTLCacheMissesUntilSet(t *testing.T) {
	c := newTTLCache(time.Minute)
	if _, ok := c.get("k"); ok {
		t.Fatal("expected a miss on an empty cache")
	}
}

func TestTTLCacheHitsBeforeExpiry(t *testing.T) {
	c := newTTLCache(time.Minute)
	c.set("k", []byte("v"))

	got, ok := c.get("k")
	if !ok {
		t.Fatal("expected a hit")
	}
	if string(got) != "v" {
		t.Errorf("got %q, want %q", got, "v")
	}
}

func TestTTLCacheMissesAfterExpiry(t *testing.T) {
	c := newTTLCache(5 * time.Millisecond)
	c.set("k", []byte("v"))

	time.Sleep(15 * time.Millisecond)

	if _, ok := c.get("k"); ok {
		t.Fatal("expected a miss once the entry expired")
	}
}

// A non-positive TTL disables caching outright, rather than silently caching
// forever -- a caller that wants an endpoint never cached (e.g. something
// that must always be fresh) sets this instead of a second code path.
func TestTTLCacheDisabledByNonPositiveTTL(t *testing.T) {
	c := newTTLCache(0)
	c.set("k", []byte("v"))

	if _, ok := c.get("k"); ok {
		t.Fatal("a zero-TTL cache must never report a hit")
	}
}
