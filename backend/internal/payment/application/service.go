package application

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/kenji-cmyk/cross-border-logistics/internal/payment/domain"
	"github.com/kenji-cmyk/cross-border-logistics/internal/payment/ports"
	"github.com/google/uuid"
)

const (
	orderStatusWaitingDeposit          = "WAITING_DEPOSIT"
	orderStatusWaitingRemainingPayment = "WAITING_REMAINING_PAYMENT"
	depositSucceededEventType          = "payment.deposit_succeeded.v1"
	remainingSucceededEventType        = "payment.remaining_balance_succeeded.v1"
)

type CreateDepositInput struct {
	OrderID string
}

type CreateRemainingBalanceInput struct {
	OrderID string
}

type SePayWebhookInput struct {
	ProviderReference string
	EventID           string
	TransferType      string
	TransferAmountVND int64
}

type eventEnvelope struct {
	EventID     string          `json:"eventId"`
	EventType   string          `json:"eventType"`
	AggregateID string          `json:"aggregateId"`
	Producer    string          `json:"producer"`
	OccurredAt  time.Time       `json:"occurredAt"`
	Data        json.RawMessage `json:"data"`
}

type paymentSucceededData struct {
	PaymentID string          `json:"paymentId"`
	OrderID   string          `json:"orderId"`
	AmountVND int64           `json:"amountVnd"`
	Currency  domain.Currency `json:"currency"`
}

type Service struct {
	repository ports.PaymentRepository
	orders     ports.OrderReader
	gateway    ports.PaymentGateway
	now        func() time.Time
}

func NewService(repository ports.PaymentRepository, orders ports.OrderReader, gateways ...ports.PaymentGateway) *Service {
	var gateway ports.PaymentGateway
	if len(gateways) > 0 {
		gateway = gateways[0]
	}
	return &Service{repository: repository, orders: orders, gateway: gateway, now: time.Now}
}

func (s *Service) CreateDeposit(ctx context.Context, input CreateDepositInput) (domain.Payment, error) {
	return s.createPayment(ctx, strings.TrimSpace(input.OrderID), domain.TypeDeposit, orderStatusWaitingDeposit)
}

func (s *Service) CreateRemainingBalance(ctx context.Context, input CreateRemainingBalanceInput) (domain.Payment, error) {
	return s.createPayment(ctx, strings.TrimSpace(input.OrderID), domain.TypeRemainingBalance, orderStatusWaitingRemainingPayment)
}

func (s *Service) createPayment(ctx context.Context, orderID string, paymentType domain.PaymentType, expectedOrderStatus string) (domain.Payment, error) {
	if _, err := uuid.Parse(orderID); err != nil {
		return domain.Payment{}, domain.ErrInvalidInput
	}
	if lookup, ok := s.repository.(ports.PaymentLookup); ok {
		if existing, err := lookup.FindByOrderIDAndType(ctx, orderID, paymentType); err == nil {
			return existing, nil
		} else if !errors.Is(err, domain.ErrPaymentNotFound) {
			return domain.Payment{}, err
		}
	}
	summary, err := s.orders.GetPaymentSummary(ctx, orderID)
	if err != nil {
		return domain.Payment{}, err
	}
	amount := summary.DepositAmountVND
	if paymentType == domain.TypeRemainingBalance {
		amount = summary.RemainingAmountVND
	}
	if summary.OrderID != orderID || amount <= 0 || summary.Status != expectedOrderStatus {
		return domain.Payment{}, domain.ErrInvalidState
	}
	now := s.now().UTC()
	id := uuid.NewString()
	transaction := ports.GatewayTransaction{Reference: "mock-" + id, HostedURL: "https://mock-payments.local/payments/" + id}
	if s.gateway != nil {
		transaction, err = s.gateway.CreateTransaction(ctx, id, amount, "VND")
		if err != nil {
			return domain.Payment{}, domain.ErrDependency
		}
	}
	payment := domain.Payment{
		ID: id, OrderID: orderID, Type: paymentType,
		AmountVND: amount, Currency: domain.CurrencyVND, Status: domain.StatusPending,
		PaymentURL:        transaction.HostedURL,
		ProviderReference: transaction.Reference, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repository.Create(ctx, payment); err != nil {
		return domain.Payment{}, err
	}
	return payment, nil
}

func (s *Service) ProcessSePayWebhook(ctx context.Context, input SePayWebhookInput) (domain.Payment, error) {
	input.ProviderReference = strings.TrimSpace(input.ProviderReference)
	input.EventID = strings.TrimSpace(input.EventID)
	input.TransferType = strings.ToLower(strings.TrimSpace(input.TransferType))
	if input.ProviderReference == "" || input.EventID == "" || input.TransferType != "in" || input.TransferAmountVND <= 0 {
		return domain.Payment{}, domain.ErrInvalidInput
	}
	lookup, ok := s.repository.(ports.PaymentLookup)
	if !ok {
		return domain.Payment{}, domain.ErrDependency
	}
	payment, err := lookup.FindByProviderReference(ctx, input.ProviderReference)
	if err != nil {
		return domain.Payment{}, err
	}
	if payment.AmountVND != input.TransferAmountVND || payment.Currency != domain.CurrencyVND {
		return domain.Payment{}, domain.ErrInvalidInput
	}
	outbox, err := makePaymentSucceededEvent(payment, s.now().UTC())
	if err != nil {
		return domain.Payment{}, err
	}
	callbacks, ok := s.repository.(ports.CallbackRepository)
	if !ok {
		return domain.Payment{}, domain.ErrDependency
	}
	result, _, err := callbacks.SucceedCallback(ctx, payment.ID, input.EventID, outbox)
	return result, err
}

func (s *Service) Get(ctx context.Context, id string) (domain.Payment, error) {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return domain.Payment{}, domain.ErrInvalidInput
	}
	return s.repository.FindByID(ctx, id)
}

func (s *Service) BuildCheckout(ctx context.Context, id string) (ports.CheckoutForm, error) {
	payment, err := s.Get(ctx, id)
	if err != nil {
		return ports.CheckoutForm{}, err
	}
	if payment.Status != domain.StatusPending {
		return ports.CheckoutForm{}, domain.ErrInvalidState
	}
	gateway, ok := s.gateway.(ports.CheckoutGateway)
	if !ok {
		return ports.CheckoutForm{}, domain.ErrCheckoutUnavailable
	}
	form, err := gateway.BuildCheckout(ctx, payment)
	if err != nil {
		return ports.CheckoutForm{}, domain.ErrCheckoutUnavailable
	}
	return form, nil
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
	outbox, err := makePaymentSucceededEvent(payment, now)
	if err != nil {
		return domain.Payment{}, err
	}
	result, _, err := s.repository.Succeed(ctx, id, outbox)
	return result, err
}

func makePaymentSucceededEvent(payment domain.Payment, now time.Time) (ports.OutboxEvent, error) {
	eventType := depositSucceededEventType
	if payment.Type == domain.TypeRemainingBalance {
		eventType = remainingSucceededEventType
	} else if payment.Type != domain.TypeDeposit {
		return ports.OutboxEvent{}, domain.ErrInvalidState
	}
	data, err := json.Marshal(paymentSucceededData{PaymentID: payment.ID, OrderID: payment.OrderID, AmountVND: payment.AmountVND, Currency: payment.Currency})
	if err != nil {
		return ports.OutboxEvent{}, err
	}
	eventID := uuid.NewString()
	payload, err := json.Marshal(eventEnvelope{EventID: eventID, EventType: eventType, AggregateID: payment.OrderID, Producer: "payment-service", OccurredAt: now, Data: data})
	if err != nil {
		return ports.OutboxEvent{}, err
	}
	return ports.OutboxEvent{ID: eventID, AggregateID: payment.OrderID, EventType: eventType, Payload: payload, CreatedAt: now}, nil
}
