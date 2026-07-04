package polymarket

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	domain "github.com/TheGh0xt/Sagittarius/internal/domain/polymarket"
	"github.com/TheGh0xt/Sagittarius/internal/domain/shared"
)

type stubProvider struct {
	event *domain.Event
	err   error
}

func (s *stubProvider) FetchEventBySlug(ctx context.Context, slug string) (*domain.Event, error) {
	return s.event, s.err
}

func (s *stubProvider) FetchEventByID(ctx context.Context, id string) (*domain.Event, error) {
	return s.event, s.err
}

func TestFetchEventByIDValidation(t *testing.T) {
	svc := NewPmService(&stubProvider{}, slog.Default())

	var invalid shared.ErrInvalidInput
	if _, err := svc.FetchEventByID(context.Background(), ""); !errors.As(err, &invalid) {
		t.Fatalf("expected ErrInvalidInput for empty id, got %v", err)
	}
	if _, err := svc.FetchEventByID(context.Background(), "not-a-number"); !errors.As(err, &invalid) {
		t.Fatalf("expected ErrInvalidInput for non-numeric id, got %v", err)
	}
}

func TestFetchEventByIDBuildsContext(t *testing.T) {
	ev := &domain.Event{ID: "42", Title: "T", Volume: 10}
	svc := NewPmService(&stubProvider{event: ev}, slog.Default())

	got, err := svc.FetchEventByID(context.Background(), "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Event.Title != "T" || got.Event.Volume != 10 {
		t.Errorf("context not built from event: %+v", got)
	}
}

func TestFetchEventByIDPropagatesError(t *testing.T) {
	svc := NewPmService(&stubProvider{err: errors.New("boom")}, slog.Default())
	if _, err := svc.FetchEventByID(context.Background(), "42"); err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchEventBySlugValidation(t *testing.T) {
	svc := NewPmService(&stubProvider{}, slog.Default())

	var invalid shared.ErrInvalidInput
	if _, err := svc.FetchEventBySlug(context.Background(), ""); !errors.As(err, &invalid) {
		t.Fatalf("expected ErrInvalidInput for empty slug, got %v", err)
	}
}
