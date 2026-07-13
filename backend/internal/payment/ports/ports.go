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
type OrderRefundAuthorizer interface {
	AuthorizeRefund(context.Context, string, string) error
}

type GatewayTransaction struct {
	Reference, HostedURL, TransactionID string
	ResultCode                          int
	Message                             string
}
type GatewayResult struct {
	TransactionID string
	ResultCode    int
	Message       string
}
type PaymentGateway interface {
	CreateTransaction(context.Context, string, string, int64, string) (GatewayTransaction, error)
	QueryTransaction(context.Context, string, string) (GatewayResult, error)
	Refund(context.Context, string, string, string, int64) (GatewayResult, error)
	QueryRefund(context.Context, string, string) (GatewayResult, error)
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

type ExtendedRepository interface {
	PaymentRepository
	FindByOrderAndType(context.Context, string, domain.PaymentType) (domain.Payment, error)
	ListByOrder(context.Context, string) ([]domain.Payment, []domain.Refund, error)
	CompleteProviderResult(context.Context, string, string, int, string, bool, OutboxEvent) (domain.Payment, bool, error)
	CreateRefund(context.Context, domain.Refund) error
	CompleteRefund(context.Context, string, string, int, string, bool, OutboxEvent) (domain.Refund, bool, error)
	ListPending(context.Context, time.Time) ([]domain.Payment, []domain.Refund, error)
}
