package handler

import (
	"context"
	"log/slog"

	"github.com/TheGh0xt/Sagittarius/internal/application/polymarket"
	"github.com/TheGh0xt/Sagittarius/internal/domain/shared"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Pmhandler struct {
	pmService polymarket.Service
	slg       *slog.Logger
}

func NewPmhandler(pmService polymarket.Service, slg *slog.Logger) *Pmhandler {
	return &Pmhandler{
		pmService: pmService,
		slg:       slg,
	}
}

// FetchEventBySlug fetches an event by its slug.
func (h *Pmhandler) FetchEventBySlug(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input polymarket.FetchEventBySlugRequest,
) (
	*mcp.CallToolResult,
	any,
	error,
) {
	eventIntelCxt, err := h.pmService.FetchEventBySlug(ctx, input.Slug)
	if err != nil {
		return nil, nil, err
	}

	result, err := shared.MarshalJSON(eventIntelCxt)
	if err != nil {
		return nil, nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(result),
			},
		},
	}, nil, nil
}

// FetchEventByID fetches an event by its numeric Gamma ID.
func (h *Pmhandler) FetchEventByID(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input polymarket.FetchEventByIDRequest,
) (
	*mcp.CallToolResult,
	any,
	error,
) {
	eventIntelCxt, err := h.pmService.FetchEventByID(ctx, input.ID)
	if err != nil {
		return nil, nil, err
	}

	result, err := shared.MarshalJSON(eventIntelCxt)
	if err != nil {
		return nil, nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(result),
			},
		},
	}, nil, nil
}

// SearchMarkets resolves free text to candidate Polymarket events.
//
// The entry point for a question rather than a slug. "Who will win the ballon
// dor?" names no market, so without this the request cannot reach one at all.
func (h *Pmhandler) SearchMarkets(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input polymarket.SearchMarketsRequest,
) (
	*mcp.CallToolResult,
	any,
	error,
) {
	matches, err := h.pmService.SearchMarkets(ctx, input)
	if err != nil {
		return nil, nil, err
	}

	result, err := shared.MarshalJSON(matches)
	if err != nil {
		return nil, nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(result),
			},
		},
	}, nil, nil
}

// GetMovingMarkets surfaces what is actually happening in a set of categories.
//
// The other tools all need a market named first. This one answers the prior
// question — which market is worth looking at — and is what lets the product
// open on something useful rather than an empty input box.
func (h *Pmhandler) GetMovingMarkets(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input polymarket.GetMovingMarketsRequest,
) (
	*mcp.CallToolResult,
	any,
	error,
) {
	markets, err := h.pmService.GetMovingMarkets(ctx, input)
	if err != nil {
		return nil, nil, err
	}

	result, err := shared.MarshalJSON(markets)
	if err != nil {
		return nil, nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(result),
			},
		},
	}, nil, nil
}
