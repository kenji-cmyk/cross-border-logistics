package ports

import (
	"context"
	"encoding/json"
	"time"

	"github.com/kenji-cmyk/cross-border-logistics/internal/order/domain"
)

type QuotationSnapshot struct {
	QuotationID    string
	CustomerID     string
	ProductURL     string
	ProductName    string
	Quantity       int
	TotalAmountVND int64
	Status         string
	CreatedAt      time.Time
}

type QuotationReader interface {
	GetQuotation(context.Context, string) (QuotationSnapshot, error)
}

type QuotationConfirmer interface {
	ConfirmQuotation(context.Context, string, string) (QuotationSnapshot, error)
}

type OutboxEvent struct {
	ID          string
	AggregateID string
	EventType   string
	Payload     json.RawMessage
	CreatedAt   time.Time
}

type OrderRepository interface {
	Create(context.Context, domain.Order, domain.TrackingEvent, OutboxEvent) error
	FindByID(context.Context, string) (domain.Order, error)
	FindTimeline(context.Context, string) ([]domain.TrackingEvent, error)
	ProcessPaymentSucceeded(context.Context, ProcessPaymentSucceeded) (bool, error)
	ProcessRemainingPaymentSucceeded(context.Context, ProcessPaymentSucceeded) (bool, error)
	ProcessPackageReceived(context.Context, ProcessPackageReceived) (bool, error)
}

type ProcessPackageReceived struct {
	EventID, EventType, OrderID string
	ProcessedAt                 time.Time
	Tracking                    domain.TrackingEvent
	Outbox                      OutboxEvent
}

type ProcessPaymentSucceeded struct {
	EventID, EventType, OrderID string
	AmountVND                   int64
	ProcessedAt                 time.Time
	Tracking                    domain.TrackingEvent
	Outbox                      OutboxEvent
}
