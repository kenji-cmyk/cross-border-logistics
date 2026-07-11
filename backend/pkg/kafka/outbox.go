package kafka

import (
	"context"
	"log/slog"
	"time"
)

type OutboxEvent struct {
	ID, AggregateID, EventType string
	Payload                    []byte
}

type OutboxStore interface {
	FetchUnpublished(context.Context, int) ([]OutboxEvent, error)
	MarkPublished(context.Context, string, time.Time) error
	MarkFailed(context.Context, string, string) error
}

type OutboxWorker struct {
	store     OutboxStore
	publisher Publisher
	interval  time.Duration
	logger    *slog.Logger
}

func NewOutboxWorker(store OutboxStore, publisher Publisher, interval time.Duration, logger *slog.Logger) *OutboxWorker {
	return &OutboxWorker{store: store, publisher: publisher, interval: interval, logger: logger}
}

func (w *OutboxWorker) Run(ctx context.Context) {
	w.poll(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.poll(ctx)
		}
	}
}

func (w *OutboxWorker) poll(ctx context.Context) {
	events, err := w.store.FetchUnpublished(ctx, 20)
	if err != nil {
		if ctx.Err() == nil {
			w.logger.ErrorContext(ctx, "outbox poll failed", "error", err)
		}
		return
	}
	for _, item := range events {
		if err := w.publisher.Publish(ctx, item.EventType, item.AggregateID, item.Payload); err != nil {
			if ctx.Err() != nil {
				return
			}
			w.logger.ErrorContext(ctx, "outbox publish failed", "event_id", item.ID, "order_id", item.AggregateID, "event_type", item.EventType, "error", err)
			if markErr := w.store.MarkFailed(ctx, item.ID, err.Error()); markErr != nil {
				w.logger.ErrorContext(ctx, "record outbox failure failed", "event_id", item.ID, "error", markErr)
			}
			continue
		}
		if err := w.store.MarkPublished(ctx, item.ID, time.Now().UTC()); err != nil {
			w.logger.ErrorContext(ctx, "mark outbox published failed", "event_id", item.ID, "error", err)
			continue
		}
		w.logger.InfoContext(ctx, "outbox event published", "event_id", item.ID, "order_id", item.AggregateID, "event_type", item.EventType)
	}
}
