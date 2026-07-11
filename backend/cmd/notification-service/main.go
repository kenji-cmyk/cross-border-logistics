package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/example/cross-border-logistics/pkg/config"
	sharedevent "github.com/example/cross-border-logistics/pkg/event"
	"github.com/example/cross-border-logistics/pkg/httpx"
	"github.com/example/cross-border-logistics/pkg/logger"
	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
)

type hub struct {
	mu          sync.RWMutex
	history     map[string][]sharedevent.Envelope
	subscribers map[string]map[chan sharedevent.Envelope]struct{}
}

func newHub() *hub {
	return &hub{history: map[string][]sharedevent.Envelope{}, subscribers: map[string]map[chan sharedevent.Envelope]struct{}{}}
}
func (h *hub) publish(e sharedevent.Envelope) {
	id := e.AggregateID.String()
	h.mu.Lock()
	items := append(h.history[id], e)
	if len(items) > 100 {
		items = items[len(items)-100:]
	}
	h.history[id] = items
	for ch := range h.subscribers[id] {
		select {
		case ch <- e:
		default:
		}
	}
	h.mu.Unlock()
}
func (h *hub) subscribe(id, last string) (chan sharedevent.Envelope, []sharedevent.Envelope, func()) {
	ch := make(chan sharedevent.Envelope, 16)
	h.mu.Lock()
	if h.subscribers[id] == nil {
		h.subscribers[id] = map[chan sharedevent.Envelope]struct{}{}
	}
	h.subscribers[id][ch] = struct{}{}
	var replay []sharedevent.Envelope
	seen := last == ""
	for _, e := range h.history[id] {
		if seen {
			replay = append(replay, e)
		} else if e.EventID.String() == last {
			seen = true
		}
	}
	h.mu.Unlock()
	return ch, replay, func() { h.mu.Lock(); delete(h.subscribers[id], ch); close(ch); h.mu.Unlock() }
}

func main() {
	cfg, err := config.Load("notification-service", "NOTIFICATION_SERVICE_PORT", "NOTIFICATION_DB")
	if err != nil {
		log.Fatal(err)
	}
	l := logger.New(cfg.ServiceName, cfg.AppEnv)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	h := newHub()
	client, err := kgo.NewClient(kgo.SeedBrokers(cfg.KafkaBrokers...), kgo.ConsumerGroup("notification-service-status-events"), kgo.ConsumeTopics(sharedevent.OrderStatusChanged), kgo.DisableAutoCommit(), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	go consume(ctx, client, h, l)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, 200, httpx.Health{Status: "UP", Service: cfg.ServiceName})
	})
	mux.HandleFunc("GET /api/v1/notifications/orders/{orderId}/stream", func(w http.ResponseWriter, r *http.Request) { stream(w, r, h) })
	if err := httpx.RunContext(ctx, l, cfg.Port, mux); err != nil {
		l.Error("service stopped", "error", err)
	}
}

func consume(ctx context.Context, c *kgo.Client, h *hub, l interface {
	ErrorContext(context.Context, string, ...any)
}) {
	for ctx.Err() == nil {
		fetches := c.PollRecords(ctx, 10)
		for _, record := range fetches.Records() {
			var e sharedevent.Envelope
			if json.Unmarshal(record.Value, &e) == nil && e.EventType == sharedevent.OrderStatusChanged {
				h.publish(e)
			} else {
				l.ErrorContext(ctx, "invalid notification event")
			}
			_ = c.CommitRecords(ctx, record)
		}
	}
}
func stream(w http.ResponseWriter, r *http.Request, h *hub) {
	id := strings.TrimSpace(r.PathValue("orderId"))
	if _, err := uuid.Parse(id); err != nil {
		httpx.WriteError(w, r, 400, "VALIDATION_ERROR", "order ID is invalid", nil)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		httpx.WriteError(w, r, 500, "STREAM_UNAVAILABLE", "streaming is unavailable", nil)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()
	ch, replay, done := h.subscribe(id, r.Header.Get("Last-Event-ID"))
	defer done()
	send := func(e sharedevent.Envelope) {
		payload, _ := e.Marshal()
		fmt.Fprintf(w, "id: %s\nevent: order.status_changed\ndata: %s\n\n", e.EventID, payload)
		flusher.Flush()
	}
	for _, e := range replay {
		send(e)
	}
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-ch:
			send(e)
		case <-ticker.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}
