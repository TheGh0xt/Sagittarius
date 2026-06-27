package polymarket

import (
	"context"
)

type EventProvider interface {
	FetchEventBySlug(ctx context.Context, slug string) (*Event, error)
}
