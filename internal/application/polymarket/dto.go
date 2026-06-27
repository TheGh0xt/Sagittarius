package polymarket

type (
	FetchEventBySlugRequest struct {
		Slug string `json:"slug" binding:"required" jsonschema:"slug of an event in polymarket is the identifier after /event/ in the URL"`
	}

	EventIntelligenceContext struct {
		Event   EventSummary     `json:"event"`
		Context string           `json:"context"`
		Tags    []string         `json:"tags"`
		Markets []MarketAnalysis `json:"markets"`
	}
)
