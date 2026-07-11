package ports

import (
	"context"
	"encoding/json"
	"time"

	"github.com/example/cross-border-logistics/internal/payment/domain"
)

type OrderPaymentSummary struct {
	OrderID            string
	DepositAmountVND   int64
	RemainingAmountVND int64
	Status             string
}

type OrderReader interface {
	GetPaymentSummary(context.Context, string) (OrderPaymentSummary, error)
}

type GatewayTransaction struct{ Reference, HostedURL string }
type PaymentGateway interface {
	CreateTransaction(context.Context, string, int64, string) (GatewayTransaction, error)
}
type PaymentLookup interface {
	FindByOrderID(context.Context, string) (domain.Payment, error)
	FindByProviderReference(context.Context, string) (domain.Payment, error)
}
type CallbackRepository interface {
	SucceedCallback(context.Context, string, string, OutboxEvent) (domain.Payment, bool, error)
}

type OutboxEvent struct {
	ID          string
	AggregateID string
	EventType   string
	Payload     json.RawMessage
	CreatedAt   time.Time
}

type PaymentRepository interface {
	Create(context.Context, domain.Payment) error
	FindByID(context.Context, string) (domain.Payment, error)
	Succeed(context.Context, string, OutboxEvent) (domain.Payment, bool, error)
}
