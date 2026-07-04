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
