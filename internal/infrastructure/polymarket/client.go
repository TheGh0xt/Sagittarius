package polymarket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/TheGh0xt/Sagittarius/internal/domain/polymarket"
)

const (
	GammaBaseURL = "https://gamma-api.polymarket.com"
	DataBaseURL  = "https://data-api.polymarket.com"
)

type pmErr struct {
	ErrType    string `json:"error"`
	ErrMessage string `json:"message"`
}

type Client struct {
	c   http.Client
	slg *slog.Logger
}

func NewClient(slg *slog.Logger) polymarket.EventProvider {
	return &Client{
		c: http.Client{
			Timeout: time.Second * 30,
		},
		slg: slg,
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

func getUrl(baseURL, path string) string {
	return fmt.Sprintf("%s%s", baseURL, path)
}

func makePmGetRequest[T any](ctx context.Context, cl *Client, url string) (*T, error) {
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
		cl.slg.Error("failed to check response status", "err", err)
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
		cl.slg.Error("failed to check response status", "err", err)
		return nil, err
	}

	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		cl.slg.Error("failed to decode response", "err", err)
		return nil, err
	}
	return &result, nil
}
