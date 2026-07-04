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
}

type pmService struct {
	ep  polymarket.EventProvider
	slg *slog.Logger
}

func NewPmService(
	ep polymarket.EventProvider,
	slg *slog.Logger,
) Service {
	return &pmService{
		ep:  ep,
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
