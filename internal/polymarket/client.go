package polymarket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	GammaBaseURL = "https://gamma-api.polymarket.com"
	ClobBaseURL  = "https://clob.polymarket.com"
	DataBaseURL  = "https://data-api.polymarket.com" // Fallbacks to Gamma/Clob where appropriate
)

type Client struct {
	httpClient *http.Client
	rateLimiter <-chan time.Time
	cache       sync.Map
	cacheTTL    time.Duration
}

type cacheEntry struct {
	value      interface{}
	expiration time.Time
}

func NewClient(reqsPerSec int, cacheTTL time.Duration) *Client {
	interval := time.Second / time.Duration(reqsPerSec)
	return &Client{
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		rateLimiter: time.Tick(interval),
		cacheTTL:    cacheTTL,
	}
}

func (c *Client) getJSON(ctx context.Context, url string, target interface{}, useCache bool) error {
	if useCache {
		if entry, ok := c.cache.Load(url); ok {
			cached := entry.(cacheEntry)
			if time.Now().Before(cached.expiration) {
				// Serialize and deserialize to return a fresh copy and avoid mutating cached objects
				data, err := json.Marshal(cached.value)
				if err == nil {
					if err := json.Unmarshal(data, target); err == nil {
						return nil
					}
				}
			}
		}
	}

	// Apply rate limiting
	select {
	case <-c.rateLimiter:
	case <-ctx.Done():
		return ctx.Err()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d from URL %s", resp.StatusCode, url)
	}

	// Decode into target
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return err
	}

	if useCache && c.cacheTTL > 0 {
		c.cache.Store(url, cacheEntry{
			value:      target,
			expiration: time.Now().Add(c.cacheTTL),
		})
	}

	return nil
}
