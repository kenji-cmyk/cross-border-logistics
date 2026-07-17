package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/kenji-cmyk/cross-border-logistics/internal/order/domain"
	sharedevent "github.com/kenji-cmyk/cross-border-logistics/pkg/event"
	"github.com/twmb/franz-go/pkg/kgo"
)

type WarehouseHandler interface {
	HandlePackageReceived(context.Context, sharedevent.Envelope) (bool, error)
}
type WarehouseConsumer struct {
	client  *kgo.Client
	handler WarehouseHandler
	logger  *slog.Logger
}

func NewWarehouseConsumer(brokers []string, group string, handler WarehouseHandler, logger *slog.Logger) (*WarehouseConsumer, error) {
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.ConsumerGroup(group), kgo.ConsumeTopics(sharedevent.PackageReceived), kgo.DisableAutoCommit(), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	if err != nil {
		return nil, fmt.Errorf("create warehouse event consumer: %w", err)
	}
	return &WarehouseConsumer{client: client, handler: handler, logger: logger}, nil
}
func (c *WarehouseConsumer) Ping(ctx context.Context) error {
	if err := c.client.Ping(ctx); err != nil {
		return fmt.Errorf("ping warehouse consumer: %w", err)
	}
	return nil
}
func (c *WarehouseConsumer) Run(ctx context.Context) {
	for ctx.Err() == nil {
		fetches := c.client.PollRecords(ctx, 1)
		if err := fetches.Err(); err != nil {
			if ctx.Err() == nil {
				c.logger.ErrorContext(ctx, "Kafka warehouse consume failed", "error", err)
			}
			continue
		}
		for _, record := range fetches.Records() {
			if !c.process(ctx, record) {
				return
			}
			if err := c.client.CommitRecords(ctx, record); err != nil && ctx.Err() == nil {
				c.logger.ErrorContext(ctx, "Kafka warehouse offset commit failed", "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "error", err)
			}
		}
	}
}
func (c *WarehouseConsumer) process(ctx context.Context, record *kgo.Record) bool {
	var envelope sharedevent.Envelope
	if err := json.Unmarshal(record.Value, &envelope); err != nil {
		c.logger.ErrorContext(ctx, "invalid warehouse Kafka event envelope", "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "error", err)
		return true
	}
	for ctx.Err() == nil {
		changed, err := c.handler.HandlePackageReceived(ctx, envelope)
		if err == nil {
			c.logger.InfoContext(ctx, "package event processed", "event_id", envelope.EventID, "order_id", envelope.AggregateID, "changed", changed)
			return true
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			c.logger.ErrorContext(ctx, "invalid package event contract; skipping poison record", "event_id", envelope.EventID, "order_id", envelope.AggregateID, "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "error", err)
			return true
		}
		c.logger.ErrorContext(ctx, "package event processing failed; retrying", "event_id", envelope.EventID, "order_id", envelope.AggregateID, "error", err)
		select {
		case <-ctx.Done():
			return false
		case <-time.After(time.Second):
		}
	}
	return false
}
func (c *WarehouseConsumer) Close() { c.client.Close() }
