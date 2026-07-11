package kafka_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	sharedkafka "github.com/example/cross-border-logistics/pkg/kafka"
)

type fakeStore struct {
	mu                sync.Mutex
	events            []sharedkafka.OutboxEvent
	published, failed int
}

func (f *fakeStore) FetchUnpublished(context.Context, int) ([]sharedkafka.OutboxEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sharedkafka.OutboxEvent(nil), f.events...), nil
}
func (f *fakeStore) MarkPublished(context.Context, string, time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.published++
	f.events = nil
	return nil
}
func (f *fakeStore) MarkFailed(context.Context, string, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failed++
	return nil
}

type fakePublisher struct {
	err               error
	failuresRemaining int
	topic, key        string
	payload           []byte
}

func (f *fakePublisher) Publish(_ context.Context, topic, key string, payload []byte) error {
	f.topic, f.key, f.payload = topic, key, payload
	if f.failuresRemaining > 0 {
		f.failuresRemaining--
		return f.err
	}
	if f.err != nil && f.failuresRemaining == 0 {
		return nil
	}
	return f.err
}

func TestOutboxWorkerRetriesOnNextPoll(t *testing.T) {
	store := &fakeStore{events: []sharedkafka.OutboxEvent{{ID: "e-1", AggregateID: "o-1", EventType: "payment.deposit_succeeded.v1", Payload: []byte(`{}`)}}}
	publisher := &fakePublisher{err: errors.New("broker unavailable"), failuresRemaining: 1}
	ctx, cancel := context.WithCancel(context.Background())
	worker := sharedkafka.NewOutboxWorker(store, publisher, time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)))
	done := make(chan struct{})
	go func() { worker.Run(ctx); close(done) }()
	for i := 0; i < 200; i++ {
		store.mu.Lock()
		published, failed := store.published, store.failed
		store.mu.Unlock()
		if published == 1 && failed == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.failed != 1 || store.published != 1 {
		t.Fatalf("failed=%d published=%d", store.failed, store.published)
	}
}

func TestOutboxWorkerPublishesWithEventTypeAndAggregateKey(t *testing.T) {
	store := &fakeStore{events: []sharedkafka.OutboxEvent{{ID: "e-1", AggregateID: "o-1", EventType: "order.created.v1", Payload: []byte(`{"eventId":"e-1"}`)}}}
	publisher := &fakePublisher{}
	ctx, cancel := context.WithCancel(context.Background())
	worker := sharedkafka.NewOutboxWorker(store, publisher, time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))
	done := make(chan struct{})
	go func() { worker.Run(ctx); close(done) }()
	for i := 0; i < 100; i++ {
		store.mu.Lock()
		count := store.published
		store.mu.Unlock()
		if count == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done
	if publisher.topic != "order.created.v1" || publisher.key != "o-1" || store.published != 1 {
		t.Fatalf("topic=%s key=%s published=%d", publisher.topic, publisher.key, store.published)
	}
}
