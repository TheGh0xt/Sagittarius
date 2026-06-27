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
}
