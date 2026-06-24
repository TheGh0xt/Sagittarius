package polymarket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

type RawMarket struct {
	ID            string `json:"id"`
	Question      string `json:"question"`
	Description   string `json:"description"`
	Slug          string `json:"slug"`
	Outcomes      string `json:"outcomes"`      // stringified array: ["Yes", "No"]
	OutcomePrices string `json:"outcomePrices"` // stringified array: ["0.55", "0.45"]
	ClobTokenIDs  string `json:"clobTokenIds"`  // stringified array of 77-digit IDs
	Active        bool   `json:"active"`
	Closed        bool   `json:"closed"`
	Volume        string `json:"volume"`
	Liquidity     string `json:"liquidity"`
}

type ParsedMarket struct {
	ID            string    `json:"id"`
	Question      string    `json:"question"`
	Description   string    `json:"description"`
	Slug          string    `json:"slug"`
	Outcomes      []string  `json:"outcomes"`
	OutcomePrices []float64 `json:"outcomePrices"`
	ClobTokenIDs  []string  `json:"clobTokenIds"`
	Active        bool      `json:"active"`
	Closed        bool      `json:"closed"`
	Volume        float64   `json:"volume"`
	Liquidity     float64   `json:"liquidity"`
}

func (rm *RawMarket) Parse() (*ParsedMarket, error) {
	pm := &ParsedMarket{
		ID:          rm.ID,
		Question:    rm.Question,
		Description: rm.Description,
		Slug:        rm.Slug,
		Active:      rm.Active,
		Closed:      rm.Closed,
	}

	if rm.Volume != "" {
		pm.Volume, _ = strconv.ParseFloat(rm.Volume, 64)
	}
	if rm.Liquidity != "" {
		pm.Liquidity, _ = strconv.ParseFloat(rm.Liquidity, 64)
	}

	if rm.Outcomes != "" && rm.Outcomes != "null" {
		_ = json.Unmarshal([]byte(rm.Outcomes), &pm.Outcomes)
	}
	if rm.ClobTokenIDs != "" && rm.ClobTokenIDs != "null" {
		_ = json.Unmarshal([]byte(rm.ClobTokenIDs), &pm.ClobTokenIDs)
	}
	if rm.OutcomePrices != "" && rm.OutcomePrices != "null" {
		var pricesStr []string
		if err := json.Unmarshal([]byte(rm.OutcomePrices), &pricesStr); err == nil {
			for _, pStr := range pricesStr {
				val, _ := strconv.ParseFloat(pStr, 64)
				pm.OutcomePrices = append(pm.OutcomePrices, val)
			}
		}
	}

	return pm, nil
}

func (c *Client) SearchMarkets(ctx context.Context, query string, limit int) ([]*ParsedMarket, error) {
	if limit <= 0 {
		limit = 20
	}
	u := fmt.Sprintf("%s/markets?active=true&closed=false&limit=%d&q=%s", GammaBaseURL, limit, url.QueryEscape(query))
	var rawMarkets []*RawMarket
	if err := c.getJSON(ctx, u, &rawMarkets, true); err != nil {
		return nil, err
	}

	var parsed []*ParsedMarket
	for _, rm := range rawMarkets {
		pm, err := rm.Parse()
		if err == nil {
			parsed = append(parsed, pm)
		}
	}
	return parsed, nil
}

func (c *Client) GetMarket(ctx context.Context, marketID string) (*ParsedMarket, error) {
	u := fmt.Sprintf("%s/markets/%s", GammaBaseURL, marketID)
	var rm RawMarket
	if err := c.getJSON(ctx, u, &rm, true); err != nil {
		return nil, err
	}
	return rm.Parse()
}
