package application

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/example/cross-border-logistics/internal/payment/domain"
	"github.com/example/cross-border-logistics/internal/payment/ports"
	"github.com/google/uuid"
)

const (
	orderStatusWaitingDeposit = "WAITING_DEPOSIT"
	depositSucceededEventType = "payment.deposit_succeeded.v1"
)

type CreateDepositInput struct {
	OrderID string
}

type eventEnvelope struct {
	EventID     string          `json:"eventId"`
	EventType   string          `json:"eventType"`
	AggregateID string          `json:"aggregateId"`
	Producer    string          `json:"producer"`
	OccurredAt  time.Time       `json:"occurredAt"`
	Data        json.RawMessage `json:"data"`
}

type depositSucceededData struct {
	PaymentID string          `json:"paymentId"`
	OrderID   string          `json:"orderId"`
	AmountVND int64           `json:"amountVnd"`
	Currency  domain.Currency `json:"currency"`
}

type Service struct {
	repository ports.PaymentRepository
	orders     ports.OrderReader
	now        func() time.Time
}

func NewService(repository ports.PaymentRepository, orders ports.OrderReader) *Service {
	return &Service{repository: repository, orders: orders, now: time.Now}
}

func (s *Service) CreateDeposit(ctx context.Context, input CreateDepositInput) (domain.Payment, error) {
	input.OrderID = strings.TrimSpace(input.OrderID)
	if _, err := uuid.Parse(input.OrderID); err != nil {
		return domain.Payment{}, domain.ErrInvalidInput
	}
	summary, err := s.orders.GetPaymentSummary(ctx, input.OrderID)
	if err != nil {
		return domain.Payment{}, err
	}
	if summary.OrderID != input.OrderID || summary.DepositAmountVND <= 0 || summary.Status != orderStatusWaitingDeposit {
		return domain.Payment{}, domain.ErrInvalidState
	}
	now := s.now().UTC()
	id := uuid.NewString()
	payment := domain.Payment{
		ID: id, OrderID: input.OrderID, Type: domain.TypeDeposit,
		AmountVND: summary.DepositAmountVND, Currency: domain.CurrencyVND, Status: domain.StatusPending,
		PaymentURL:        "https://mock-payments.local/payments/" + id,
		ProviderReference: "mock-" + id, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repository.Create(ctx, payment); err != nil {
		return domain.Payment{}, err
	}
	return payment, nil
}

func (s *Service) Get(ctx context.Context, id string) (domain.Payment, error) {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return domain.Payment{}, domain.ErrInvalidInput
	}
	return s.repository.FindByID(ctx, id)
}

func (s *Service) MarkSucceeded(ctx context.Context, id string) (domain.Payment, error) {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return domain.Payment{}, domain.ErrInvalidInput
	}
	payment, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return domain.Payment{}, err
	}
	if payment.Status == domain.StatusSucceeded {
		return payment, nil
	}
	if !domain.CanTransition(payment.Status, domain.StatusSucceeded) {
		return domain.Payment{}, domain.ErrInvalidState
	}
	now := s.now().UTC()
	outbox, err := makeDepositSucceededEvent(payment, now)
	if err != nil {
		return domain.Payment{}, err
	}
	result, _, err := s.repository.Succeed(ctx, id, outbox)
	return result, err
}

func makeDepositSucceededEvent(payment domain.Payment, now time.Time) (ports.OutboxEvent, error) {
	data, err := json.Marshal(depositSucceededData{PaymentID: payment.ID, OrderID: payment.OrderID, AmountVND: payment.AmountVND, Currency: payment.Currency})
	if err != nil {
		return ports.OutboxEvent{}, err
	}
	eventID := uuid.NewString()
	payload, err := json.Marshal(eventEnvelope{EventID: eventID, EventType: depositSucceededEventType, AggregateID: payment.OrderID, Producer: "payment-service", OccurredAt: now, Data: data})
	if err != nil {
		return ports.OutboxEvent{}, err
	}
	return ports.OutboxEvent{ID: eventID, AggregateID: payment.OrderID, EventType: depositSucceededEventType, Payload: payload, CreatedAt: now}, nil
}
