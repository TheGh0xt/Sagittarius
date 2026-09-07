package polymarket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/TheGh0xt/Sagittarius/internal/domain/polymarket"
	"golang.org/x/time/rate"
)

const (
	GammaBaseURL = "https://gamma-api.polymarket.com"
	DataBaseURL  = "https://data-api.polymarket.com"
	// ClobBaseURL is the public CLOB host. Note: the spec doc's
	// clob-api.polymarket.com does not resolve; clob.polymarket.com is the
	// real public endpoint.
	ClobBaseURL = "https://clob.polymarket.com"

	// defaultCacheTTL is how long a GET response is served from cache before
	// being refetched. Short enough that a signal (price, orderbook, whale
	// trade) is never stale by more than this; long enough that a default
	// get_moving_markets call -- 14 Gamma requests across the 13 product
	// categories -- and any repeat of it within the window pay for those
	// requests once rather than every time.
	defaultCacheTTL = 20 * time.Second

	// defaultRatePerSecond and defaultBurst bound outbound requests to every
	// Polymarket host this client talks to. Gamma, Data and CLOB publish no
	// documented public rate limit, so this is a conservative, good-neighbor
	// ceiling rather than a measured one -- comfortably above what a single
	// get_moving_markets fan-out needs once caching removes the duplicate
	// requests, and low enough to stay well clear of anything Polymarket
	// might impose without notice.
	defaultRatePerSecond = 8
	defaultBurst         = 8
)

type pmErr struct {
	ErrType    any `json:"error"`
	ErrMessage any `json:"message"`
}

type Client struct {
	c   http.Client
	slg *slog.Logger

	baseGammaURL string
	baseDataURL  string
	baseClobURL  string

	cache   *ttlCache
	limiter *rate.Limiter
}

// Compile-time checks that Client satisfies the domain provider interfaces.
var _ polymarket.EventProvider = (*Client)(nil)

func NewClient(slg *slog.Logger) *Client {
	return NewClientWithBaseURLs(slg, GammaBaseURL, DataBaseURL, ClobBaseURL)
}

// NewClientWithBaseURLs returns a Client pointed at custom API base URLs.
// Production code should use NewClient; this exists for httptest-backed tests.
func NewClientWithBaseURLs(slg *slog.Logger, gammaURL, dataURL, clobURL string) *Client {
	return &Client{
		c: http.Client{
			Timeout: time.Second * 30,
		},
		slg:          slg,
		baseGammaURL: gammaURL,
		baseDataURL:  dataURL,
		baseClobURL:  clobURL,
		cache:        newTTLCache(defaultCacheTTL),
		limiter:      rate.NewLimiter(rate.Limit(defaultRatePerSecond), defaultBurst),
	}
}

func decodePmErr(status int, body []byte) error {
	var perr pmErr
	if err := json.Unmarshal(body, &perr); err != nil {
		return fmt.Errorf("upstream status %d: %s", status, string(body))
	}
	return fmt.Errorf("%v:%v", perr.ErrType, perr.ErrMessage)
}

func (c *Client) gammaEventBySlugURL(slug string) string {
	return fmt.Sprintf("%s/events/slug/%s", c.baseGammaURL, url.PathEscape(slug))
}

func (c *Client) gammaEventByIDURL(id string) string {
	return fmt.Sprintf("%s/events/%s", c.baseGammaURL, url.PathEscape(id))
}

// gammaSearchURL builds a public-search query for live events only.
//
// events_status=active is load-bearing: without it Gamma returns settled
// events from previous years alongside current ones, and the top hit for a
// recurring event is usually last year's closed market.
func (c *Client) gammaSearchURL(query string, limit int) string {
	params := url.Values{}
	params.Set("q", query)
	params.Set("events_status", "active")
	params.Set("limit_per_type", strconv.Itoa(limit))
	return fmt.Sprintf("%s/public-search?%s", c.baseGammaURL, params.Encode())
}

// doRequest performs one logical HTTP call, applying outbound rate limiting
// and retrying transient failures (network errors and 5xx responses) with
// exponential backoff. It returns the raw response body on a 2xx status.
//
// A fresh *http.Request is built on every attempt rather than one reused
// across retries: an http.Request whose body has already been read cannot be
// resent safely, and rebuilding is cheap next to a network round trip.
func (c *Client) doRequest(ctx context.Context, method, reqURL string, body []byte) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			if err := c.sleep(ctx, backoff(attempt-1)); err != nil {
				return nil, err
			}
		}

		if err := c.limiter.Wait(ctx); err != nil {
			return nil, err
		}

		respBody, status, err := c.attempt(ctx, method, reqURL, body)
		if err != nil {
			lastErr = err
			c.slg.Warn("request failed; will retry if attempts remain",
				"url", reqURL, "attempt", attempt+1, "max_attempts", maxAttempts, "err", err)
			continue
		}

		if status != http.StatusOK {
			lastErr = decodePmErr(status, respBody)
			if !isRetryableStatus(status) || attempt == maxAttempts-1 {
				c.slg.Error("upstream returned an error status", "url", reqURL, "status", status)
				return nil, lastErr
			}
			c.slg.Warn("upstream returned a retryable status",
				"url", reqURL, "status", status, "attempt", attempt+1, "max_attempts", maxAttempts)
			continue
		}

		return respBody, nil
	}

	return nil, fmt.Errorf("request to %s failed after %d attempts: %w", reqURL, maxAttempts, lastErr)
}

// attempt performs a single HTTP round trip, with no retry logic of its own.
func (c *Client) attempt(ctx context.Context, method, reqURL string, body []byte) ([]byte, int, error) {
	var payload io.Reader
	if body != nil {
		payload = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, payload)
	if err != nil {
		return nil, 0, fmt.Errorf("building request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.c.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("performing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading response body: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

// sleep waits for d, or returns ctx's error if it is cancelled first.
func (c *Client) sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// makePmGetRequest performs a cached, rate-limited, retried GET and decodes
// the JSON response as T.
//
// Cached by full URL, which already encodes the endpoint and every query
// parameter, so distinct queries against the same endpoint (e.g. two
// different tag slugs) are never confused with one another.
func makePmGetRequest[T any](ctx context.Context, cl *Client, reqURL string) (*T, error) {
	if cached, ok := cl.cache.get(reqURL); ok {
		var result T
		if err := json.Unmarshal(cached, &result); err == nil {
			return &result, nil
		}
		// A corrupt cache entry (should not happen; defensive) falls through
		// to a real fetch rather than failing the call.
		cl.slg.Warn("cache entry failed to decode; refetching", "url", reqURL)
	}

	body, err := cl.doRequest(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	cl.cache.set(reqURL, body)

	var result T
	if err := json.Unmarshal(body, &result); err != nil {
		cl.slg.Error("failed to decode response", "url", reqURL, "err", err)
		return nil, err
	}
	return &result, nil
}

// makePmPostRequest performs a rate-limited, retried POST and decodes the
// JSON response as T. Not cached: POSTs are not assumed idempotent.
func makePmPostRequest[T any](ctx context.Context, cl *Client, reqURL string, body []byte) (*T, error) {
	respBody, err := cl.doRequest(ctx, http.MethodPost, reqURL, body)
	if err != nil {
		return nil, err
	}

	var result T
	if err := json.Unmarshal(respBody, &result); err != nil {
		cl.slg.Error("failed to decode response", "url", reqURL, "err", err)
		return nil, err
	}
	return &result, nil
}
