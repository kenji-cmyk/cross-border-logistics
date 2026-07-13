package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	sharedevent "github.com/example/cross-border-logistics/pkg/event"
	"github.com/twmb/franz-go/pkg/kgo"
)

type Handler interface {
	HandlePaymentDepositSucceeded(context.Context, sharedevent.Envelope) (bool, error)
	HandlePaymentRemainingSucceeded(context.Context, sharedevent.Envelope) (bool, error)
	HandlePaymentRefundSucceeded(context.Context, sharedevent.Envelope) (bool, error)
}

type Consumer struct {
	client  *kgo.Client
	handler Handler
	logger  *slog.Logger
}

func NewConsumer(brokers []string, group string, handler Handler, logger *slog.Logger) (*Consumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(sharedevent.PaymentDepositSucceeded, sharedevent.PaymentRemainingSucceeded, sharedevent.PaymentRefundSucceeded),
		kgo.DisableAutoCommit(),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, fmt.Errorf("create order Kafka consumer: %w", err)
	}
	return &Consumer{client: client, handler: handler, logger: logger}, nil
}

func (c *Consumer) Ping(ctx context.Context) error {
	if err := c.client.Ping(ctx); err != nil {
		return fmt.Errorf("ping Kafka consumer: %w", err)
	}
	return nil
}

func (c *Consumer) Run(ctx context.Context) {
	for ctx.Err() == nil {
		fetches := c.client.PollRecords(ctx, 1)
		if err := fetches.Err(); err != nil {
			if ctx.Err() == nil {
				c.logger.ErrorContext(ctx, "Kafka consume failed", "error", err)
			}
			continue
		}
		for _, record := range fetches.Records() {
			if !c.processUntilSuccess(ctx, record) {
				return
			}
			if err := c.client.CommitRecords(ctx, record); err != nil && ctx.Err() == nil {
				c.logger.ErrorContext(ctx, "Kafka offset commit failed", "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "error", err)
			}
		}
	}
}

func (c *Consumer) processUntilSuccess(ctx context.Context, record *kgo.Record) bool {
	for ctx.Err() == nil {
		var envelope sharedevent.Envelope
		if err := json.Unmarshal(record.Value, &envelope); err != nil {
			c.logger.ErrorContext(ctx, "invalid Kafka event envelope", "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "error", err)
			return true
		}
		var changed bool
		var err error
		switch envelope.EventType {
		case sharedevent.PaymentDepositSucceeded:
			changed, err = c.handler.HandlePaymentDepositSucceeded(ctx, envelope)
		case sharedevent.PaymentRemainingSucceeded:
			changed, err = c.handler.HandlePaymentRemainingSucceeded(ctx, envelope)
		case sharedevent.PaymentRefundSucceeded:
			changed, err = c.handler.HandlePaymentRefundSucceeded(ctx, envelope)
		default:
			return true
		}
		if err == nil {
			c.logger.InfoContext(ctx, "payment event processed", "event_id", envelope.EventID, "order_id", envelope.AggregateID, "changed", changed)
			return true
		}
		c.logger.ErrorContext(ctx, "payment event processing failed; retrying", "event_id", envelope.EventID, "order_id", envelope.AggregateID, "error", err)
		select {
		case <-ctx.Done():
			return false
		case <-time.After(time.Second):
		}
	}
	return false
}

func (c *Consumer) Close() { c.client.Close() }
