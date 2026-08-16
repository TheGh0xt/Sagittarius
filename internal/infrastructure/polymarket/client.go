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
)

const (
	GammaBaseURL = "https://gamma-api.polymarket.com"
	DataBaseURL  = "https://data-api.polymarket.com"
	// ClobBaseURL is the public CLOB host. Note: the spec doc's
	// clob-api.polymarket.com does not resolve; clob.polymarket.com is the
	// real public endpoint.
	ClobBaseURL = "https://clob.polymarket.com"
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
	}
}

func checkRespStatusCode(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		var err pmErr
		if err := json.NewDecoder(resp.Body).Decode(&err); err != nil {
			return err
		}
		return fmt.Errorf("%s:%s", err.ErrType, err.ErrMessage)
	}
	return nil
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

func makePmGetRequest[T any](ctx context.Context, cl *Client, url string) (*T, error) {
	slog.Info("url string", "url", url)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		cl.slg.Error("failed to create request", "err", err)
		return nil, err
	}
	resp, err := cl.c.Do(req)
	if err != nil {
		cl.slg.Error("failed to make request", "err", err)
		return nil, err
	}
	defer resp.Body.Close()

	if err := checkRespStatusCode(resp); err != nil {
		cl.slg.Error("failed response status check", "err", err)
		return nil, err
	}

	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		cl.slg.Error("failed to decode response", "err", err)
		return nil, err
	}
	return &result, nil
}

func makePmPostRequest[T any](ctx context.Context, cl *Client, url string, body []byte) (*T, error) {
	var payload io.Reader
	if body != nil {
		cl.slg.Debug("making post request", "url", url, "body", string(body))
		payload = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, payload)
	if err != nil {
		cl.slg.Error("failed to create request", "err", err)
		return nil, err
	}
	resp, err := cl.c.Do(req)
	if err != nil {
		cl.slg.Error("failed to make request", "err", err)
		return nil, err
	}
	defer resp.Body.Close()

	if err := checkRespStatusCode(resp); err != nil {
		cl.slg.Error("failed response status check", "err", err)
		return nil, err
	}

	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		cl.slg.Error("failed to decode response", "err", err)
		return nil, err
	}
	return &result, nil
}
