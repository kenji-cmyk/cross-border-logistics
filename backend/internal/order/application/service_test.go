package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/example/cross-border-logistics/internal/order/application"
	"github.com/example/cross-border-logistics/internal/order/domain"
	"github.com/example/cross-border-logistics/internal/order/ports"
)

const quotationID = "46ab7a1a-bab7-4a46-b9f9-d7572a284895"

type fakeQuotationReader struct {
	snapshot ports.QuotationSnapshot
	err      error
}

func (f fakeQuotationReader) GetQuotation(context.Context, string) (ports.QuotationSnapshot, error) {
	return f.snapshot, f.err
}

type fakeRepository struct {
	order     domain.Order
	tracking  domain.TrackingEvent
	outbox    ports.OutboxEvent
	createErr error
	timeline  []domain.TrackingEvent
}

func (f *fakeRepository) Create(_ context.Context, order domain.Order, tracking domain.TrackingEvent, outbox ports.OutboxEvent) error {
	f.order, f.tracking, f.outbox = order, tracking, outbox
	return f.createErr
}
func (f *fakeRepository) FindByID(_ context.Context, id string) (domain.Order, error) {
	if f.order.ID != id {
		return domain.Order{}, domain.ErrOrderNotFound
	}
	return f.order, nil
}
func (f *fakeRepository) FindTimeline(_ context.Context, id string) ([]domain.TrackingEvent, error) {
	if f.order.ID != id {
		return nil, domain.ErrOrderNotFound
	}
	return f.timeline, nil
}

func validSnapshot() ports.QuotationSnapshot {
	return ports.QuotationSnapshot{QuotationID: quotationID, CustomerID: "customer-001", ProductURL: "https://example.com/p/1", ProductName: "Keyboard", Quantity: 1, TotalAmountVND: 1_485_000, Status: "PENDING_CONFIRMATION", CreatedAt: time.Now()}
}

func TestCreateOrderBuildsAggregateTrackingAndOutbox(t *testing.T) {
	repository := &fakeRepository{}
	service := application.NewService(repository, fakeQuotationReader{snapshot: validSnapshot()})
	result, err := service.Create(context.Background(), application.CreateInput{QuotationID: quotationID, CustomerID: "customer-001", DeliveryAddress: "Ho Chi Minh City"})
	if err != nil {
		t.Fatal(err)
	}
	if result.DepositAmountVND != 1_039_500 || result.RemainingAmountVND != 445_500 {
		t.Fatalf("unexpected amounts: %+v", result)
	}
	if result.Status != domain.StatusWaitingDeposit || len(result.Items) != 1 {
		t.Fatalf("unexpected aggregate: %+v", result)
	}
	if repository.tracking.Status != domain.StatusWaitingDeposit || repository.tracking.Description != domain.InitialTrackingDescription {
		t.Fatalf("unexpected tracking: %+v", repository.tracking)
	}
	if repository.outbox.EventType != "order.created.v1" || repository.outbox.AggregateID != result.ID {
		t.Fatalf("unexpected outbox: %+v", repository.outbox)
	}
	var payload map[string]any
	if err := json.Unmarshal(repository.outbox.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["eventType"] != "order.created.v1" || payload["producer"] != "order-service" {
		t.Fatalf("unexpected payload: %s", repository.outbox.Payload)
	}
}

func TestCreateOrderRejectsInvalidQuotation(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*ports.QuotationSnapshot)
		customer string
		want     error
	}{
		{"wrong owner", func(*ports.QuotationSnapshot) {}, "customer-002", domain.ErrCustomerMismatch},
		{"expired", func(q *ports.QuotationSnapshot) { q.Status = "EXPIRED" }, "customer-001", domain.ErrInvalidQuotation},
		{"confirmed", func(q *ports.QuotationSnapshot) { q.Status = "CONFIRMED" }, "customer-001", domain.ErrInvalidQuotation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := validSnapshot()
			tt.mutate(&snapshot)
			repository := &fakeRepository{}
			_, err := application.NewService(repository, fakeQuotationReader{snapshot: snapshot}).Create(context.Background(), application.CreateInput{QuotationID: quotationID, CustomerID: tt.customer, DeliveryAddress: "address"})
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
			if repository.order.ID != "" {
				t.Fatal("order was persisted")
			}
		})
	}
}

func TestPaymentSummary(t *testing.T) {
	repository := &fakeRepository{order: domain.Order{ID: quotationID, DepositAmountVND: 700, RemainingAmountVND: 300, Status: domain.StatusWaitingDeposit}}
	summary, err := application.NewService(repository, fakeQuotationReader{}).GetPaymentSummary(context.Background(), quotationID)
	if err != nil {
		t.Fatal(err)
	}
	if summary.DepositAmountVND != 700 || summary.RemainingAmountVND != 300 || summary.Status != domain.StatusWaitingDeposit {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}
