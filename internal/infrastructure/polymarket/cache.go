package polymarket

import (
	"sync"
	"time"
)

// ttlCache is a small in-memory cache for raw API response bodies, keyed by
// request URL — which already encodes both the endpoint and its query
// parameters, so different endpoints (and different queries against the same
// endpoint, e.g. one tag slug vs another) are cached independently.
//
// This exists because a default get_moving_markets call fans out one Gamma
// request per tag slug (14 of them across the 13 product categories), and
// every one of those degrades to the same handful of URLs across repeated or
// concurrent callers. Without a cache, every call pays for all 14 requests
// again; with one, only the first caller in a TTL window does.
//
// Deliberately not a response-type cache: caching raw bytes keyed by URL
// keeps this single cache usable from every typed call site in the package
// (makePmGetRequest is generic), rather than needing one cache per decoded
// type.
type ttlCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]cacheEntry
}

type cacheEntry struct {
	body    []byte
	expires time.Time
}

// newTTLCache returns a cache whose entries expire ttl after being written.
// A non-positive ttl disables caching: get always misses and set is a no-op,
// which lets a caller opt an endpoint out of caching without a second code
// path.
func newTTLCache(ttl time.Duration) *ttlCache {
	return &ttlCache{
		ttl:     ttl,
		entries: make(map[string]cacheEntry),
	}
}

func (c *ttlCache) get(key string) ([]byte, bool) {
	if c.ttl <= 0 {
		return nil, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expires) {
		delete(c.entries, key)
		return nil, false
	}
	return entry.body, true
}

func (c *ttlCache) set(key string, body []byte) {
	if c.ttl <= 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = cacheEntry{
		body:    body,
		expires: time.Now().Add(c.ttl),
	}
}
