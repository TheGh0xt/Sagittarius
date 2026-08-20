package polymarket

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/TheGh0xt/Sagittarius/internal/domain/polymarket"
	"github.com/TheGh0xt/Sagittarius/internal/domain/shared"
)

type Service interface {
	FetchEventBySlug(ctx context.Context, slug string) (*EventIntelligenceContext, error)
	FetchEventByID(ctx context.Context, id string) (*EventIntelligenceContext, error)
	SearchMarkets(ctx context.Context, req SearchMarketsRequest) (*SearchMarketsResponse, error)
	GetMovingMarkets(ctx context.Context, req GetMovingMarketsRequest) (*GetMovingMarketsResponse, error)
}

type pmService struct {
	ep polymarket.EventProvider
	// Separate from ep because the two answer different questions: ep fetches
	// an event you can already name, sp works out which event was meant.
	sp polymarket.SearchProvider
	// And a third question again: dp asks what is moving in an area nobody has
	// named. May be nil, in which case GetMovingMarkets reports that discovery
	// is unconfigured rather than panicking.
	dp  polymarket.DiscoveryProvider
	slg *slog.Logger
}

func NewPmService(
	ep polymarket.EventProvider,
	sp polymarket.SearchProvider,
	slg *slog.Logger,
) Service {
	return &pmService{
		ep:  ep,
		sp:  sp,
		slg: slg,
	}
}

// NewPmServiceWithDiscovery additionally wires category-based discovery, which
// backs the personalised feed and the automated report generator.
func NewPmServiceWithDiscovery(
	ep polymarket.EventProvider,
	sp polymarket.SearchProvider,
	dp polymarket.DiscoveryProvider,
	slg *slog.Logger,
) Service {
	return &pmService{
		ep:  ep,
		sp:  sp,
		dp:  dp,
		slg: slg,
	}
}

func (pms *pmService) FetchEventBySlug(ctx context.Context, slug string) (*EventIntelligenceContext, error) {
	if slug == "" {
		return nil, shared.ErrInvalidInput{Field: "event slug", Message: "event slug cannot be empty"}
	}

	event, err := pms.ep.FetchEventBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	return BuildEventIntelligenceContext(event), nil
}

func (pms *pmService) FetchEventByID(ctx context.Context, id string) (*EventIntelligenceContext, error) {
	if id == "" {
		return nil, shared.ErrInvalidInput{Field: "event id", Message: "event id cannot be empty"}
	}
	if _, err := strconv.Atoi(id); err != nil {
		return nil, shared.ErrInvalidInput{Field: "event id", Message: "event id must be numeric"}
	}

	event, err := pms.ep.FetchEventByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return BuildEventIntelligenceContext(event), nil
}
