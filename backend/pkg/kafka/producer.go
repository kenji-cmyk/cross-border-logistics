package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

type Publisher interface {
	Publish(context.Context, string, string, []byte) error
}

type Producer struct{ client *kgo.Client }

type Readiness interface{ Ping(context.Context) error }

func WaitReady(ctx context.Context, target Readiness, attempts int, backoff time.Duration) error {
	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err = target.Ping(pingCtx)
		cancel()
		if err == nil {
			return nil
		}
		if attempt < attempts {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
	}
	return fmt.Errorf("Kafka unavailable after %d attempts: %w", attempts, err)
}

func NewProducer(brokers []string) (*Producer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.RecordRetries(5),
		kgo.RetryBackoffFn(func(int) time.Duration { return 500 * time.Millisecond }),
	)
	if err != nil {
		return nil, fmt.Errorf("create Kafka producer: %w", err)
	}
	return &Producer{client: client}, nil
}

func (p *Producer) Ping(ctx context.Context) error {
	if err := p.client.Ping(ctx); err != nil {
		return fmt.Errorf("ping Kafka: %w", err)
	}
	return nil
}

func (p *Producer) Publish(ctx context.Context, topic, key string, payload []byte) error {
	result := p.client.ProduceSync(ctx, &kgo.Record{Topic: topic, Key: []byte(key), Value: payload})
	if err := result.FirstErr(); err != nil {
		return fmt.Errorf("publish Kafka record: %w", err)
	}
	return nil
}

func (p *Producer) Close() { p.client.Close() }
