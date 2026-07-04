package mcp

import "github.com/modelcontextprotocol/go-sdk/mcp"

/*
// Potential MCP tools list for Polymarkets Intelligence.
get_event_context()
get_market_prices()
get_market_history()
get_market_trades()
get_whale_activity()
get_volume_analysis()
*/

// RegisterPmiTools registers the Polymarkets Intelligence tools with the MCP server.
func (s *Server) RegisterPmiTools() {
	mcp.AddTool(
		s.ms, &mcp.Tool{
			Name:        "get_event_by_slug",
			Description: "Get Polymarkets event intelligence by slug",
		},
		s.ph.FetchEventBySlug,
	)

	mcp.AddTool(
		s.ms, &mcp.Tool{
			Name:        "get_event_by_id",
			Description: "Get Polymarkets event intelligence by numeric event ID",
		},
		s.ph.FetchEventByID,
	)

	mcp.AddTool(
		s.ms, &mcp.Tool{
			Name:        "get_whale_activity",
			Description: "Detect whale-sized trades (notional >= usd_threshold) for every market in a Polymarket event, aggregated per market with buy/sell ratio",
		},
		s.sh.DetectWhaleActivity,
	)

	mcp.AddTool(
		s.ms, &mcp.Tool{
			Name:        "get_market_snapshot",
			Description: "Unified deterministic state vector per market of a Polymarket event: implied probability, orderbook skew, volume-spike analysis, whale count",
		},
		s.sh.BuildMarketSnapshot,
	)
}
