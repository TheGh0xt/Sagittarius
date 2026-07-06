package handler

import (
	"context"
	"log/slog"

	"github.com/TheGh0xt/Sagittarius/internal/application/signal"
	"github.com/TheGh0xt/Sagittarius/internal/domain/shared"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type SignalHandler struct {
	svc signal.Service
	slg *slog.Logger
}

func NewSignalHandler(svc signal.Service, slg *slog.Logger) *SignalHandler {
	return &SignalHandler{
		svc: svc,
		slg: slg,
	}
}

// DetectWhaleActivity returns whale-sized trades per market of an event.
func (h *SignalHandler) DetectWhaleActivity(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input signal.WhaleActivityRequest,
) (
	*mcp.CallToolResult,
	any,
	error,
) {
	report, err := h.svc.DetectWhaleActivity(ctx, input)
	if err != nil {
		return nil, nil, err
	}

	result, err := shared.MarshalJSON(report)
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

// BuildMarketSnapshot returns the unified deterministic state vector per market.
func (h *SignalHandler) BuildMarketSnapshot(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input signal.MarketSnapshotRequest,
) (
	*mcp.CallToolResult,
	any,
	error,
) {
	report, err := h.svc.BuildMarketSnapshot(ctx, input.Slug)
	if err != nil {
		return nil, nil, err
	}

	result, err := shared.MarshalJSON(report)
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
