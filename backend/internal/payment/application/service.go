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

type FinancialSummary struct {
	OrderID     string           `json:"orderId"`
	Payments    []domain.Payment `json:"payments"`
	Refunds     []domain.Refund  `json:"refunds"`
	PaidVND     int64            `json:"paidVnd"`
	RefundedVND int64            `json:"refundedVnd"`
	NetPaidVND  int64            `json:"netPaidVnd"`
}

func (s *Service) RefundAll(ctx context.Context, orderID, token string) (FinancialSummary, error) {
	auth, ok := s.orders.(ports.OrderRefundAuthorizer)
	if !ok {
		return FinancialSummary{}, domain.ErrDependency
	}
	if err := auth.AuthorizeRefund(ctx, orderID, token); err != nil {
		return FinancialSummary{}, err
	}
	repo, ok := s.repository.(ports.ExtendedRepository)
	if !ok {
		return FinancialSummary{}, domain.ErrDependency
	}
	ps, rs, err := repo.ListByOrder(ctx, orderID)
	if err != nil {
		return FinancialSummary{}, err
	}
	existing := map[string]bool{}
	for _, r := range rs {
		existing[r.PaymentID] = true
	}
	for _, p := range ps {
		if p.Status != domain.StatusSucceeded || existing[p.ID] {
			continue
		}
		now := s.now().UTC()
		x := domain.Refund{ID: uuid.NewString(), PaymentID: p.ID, OrderID: orderID, AmountVND: p.AmountVND, Status: domain.StatusPending, Provider: p.Provider, ProviderRequestID: uuid.NewString(), CreatedAt: now, UpdatedAt: now}
		if err = repo.CreateRefund(ctx, x); err != nil {
			return FinancialSummary{}, err
		}
		result, e := s.gateway.Refund(ctx, x.ID, x.ProviderRequestID, p.ProviderTransID, x.AmountVND)
		if e != nil {
			continue
		}
		if result.ResultCode == 1000 || result.ResultCode == 7002 {
			continue
		}
		success := result.ResultCode == 0
		outbox := ports.OutboxEvent{}
		if success {
			outbox, e = makeRefundEvent(x, p.AmountVND, now)
			if e != nil {
				return FinancialSummary{}, e
			}
		}
		_, _, err = repo.CompleteRefund(ctx, x.ID, result.TransactionID, result.ResultCode, result.Message, success, outbox)
		if err != nil {
			return FinancialSummary{}, err
		}
	}
	return s.financial(ctx, orderID)
}
func makeRefundEvent(x domain.Refund, total int64, now time.Time) (ports.OutboxEvent, error) {
	data, _ := json.Marshal(map[string]any{"refundId": x.ID, "paymentId": x.PaymentID, "orderId": x.OrderID, "amountVnd": x.AmountVND, "totalRefundedVnd": total})
	id := uuid.NewString()
	payload, err := json.Marshal(eventEnvelope{EventID: id, EventType: "payment.refund_succeeded.v1", AggregateID: x.OrderID, Producer: "payment-service", OccurredAt: now, Data: data})
	return ports.OutboxEvent{ID: id, AggregateID: x.OrderID, EventType: "payment.refund_succeeded.v1", Payload: payload, CreatedAt: now}, err
}
func (s *Service) Reconcile(ctx context.Context) error {
	repo, ok := s.repository.(ports.ExtendedRepository)
	if !ok {
		return domain.ErrDependency
	}
	ps, rs, err := repo.ListPending(ctx, s.now().UTC().Add(-30*time.Second))
	if err != nil {
		return err
	}
	for _, p := range ps {
		result, e := s.gateway.QueryTransaction(ctx, p.ID, p.ProviderRequestID)
		if e != nil {
			continue
		}
		if result.ResultCode == 0 {
			_, _ = s.ApplyProviderResult(ctx, p.ID, p.ProviderRequestID, result.TransactionID, result.ResultCode, result.Message)
		} else if result.ResultCode != 1000 && result.ResultCode != 9000 {
			_, _ = s.ApplyProviderResult(ctx, p.ID, p.ProviderRequestID, result.TransactionID, result.ResultCode, result.Message)
		}
	}
	for _, x := range rs {
		result, e := s.gateway.QueryRefund(ctx, x.ID, x.ProviderRequestID)
		if e != nil {
			continue
		}
		success := result.ResultCode == 0
		if !success && (result.ResultCode == 1000 || result.ResultCode == 7002) {
			continue
		}
		out := ports.OutboxEvent{}
		if success {
			out, _ = makeRefundEvent(x, x.AmountVND, s.now().UTC())
		}
		_, _, _ = repo.CompleteRefund(ctx, x.ID, result.TransactionID, result.ResultCode, result.Message, success, out)
	}
	return nil
}

func (s *Service) RunReconciliation(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.Reconcile(ctx)
		}
	}
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
	input.OrderID = strings.TrimSpace(input.OrderID)
	if _, err := uuid.Parse(input.OrderID); err != nil {
		return domain.Payment{}, domain.ErrInvalidInput
	}
	if lookup, ok := s.repository.(ports.PaymentLookup); ok {
		if existing, err := lookup.FindByOrderID(ctx, input.OrderID); err == nil {
			return existing, nil
		}
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
	transaction := ports.GatewayTransaction{Reference: "mock-" + id, HostedURL: "https://mock-payments.local/payments/" + id}
	if s.gateway != nil {
		transaction, err = s.gateway.CreateTransaction(ctx, id, id, summary.DepositAmountVND, "VND")
		if err != nil {
			return domain.Payment{}, domain.ErrDependency
		}
	}
	provider := "momo"
	if strings.HasPrefix(transaction.Reference, "mock-") {
		provider = "mock"
	}
	payment := domain.Payment{
		ID: id, OrderID: input.OrderID, Type: domain.TypeDeposit,
		AmountVND: summary.DepositAmountVND, Currency: domain.CurrencyVND, Status: domain.StatusPending,
		PaymentURL:        transaction.HostedURL,
		ProviderReference: transaction.Reference, Provider: provider, ProviderRequestID: id, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repository.Create(ctx, payment); err != nil {
		return domain.Payment{}, err
	}
	return payment, nil
}

func (s *Service) CreateRemaining(ctx context.Context, input CreateDepositInput) (domain.Payment, error) {
	input.OrderID = strings.TrimSpace(input.OrderID)
	if _, err := uuid.Parse(input.OrderID); err != nil {
		return domain.Payment{}, domain.ErrInvalidInput
	}
	repo, ok := s.repository.(ports.ExtendedRepository)
	if !ok {
		return domain.Payment{}, domain.ErrDependency
	}
	if p, err := repo.FindByOrderAndType(ctx, input.OrderID, domain.TypeRemainingBalance); err == nil {
		return p, nil
	}
	deposit, err := repo.FindByOrderAndType(ctx, input.OrderID, domain.TypeDeposit)
	if err != nil || deposit.Status != domain.StatusSucceeded {
		return domain.Payment{}, domain.ErrInvalidState
	}
	summary, err := s.orders.GetPaymentSummary(ctx, input.OrderID)
	if err != nil {
		return domain.Payment{}, err
	}
	if summary.RemainingAmountVND <= 0 || summary.Status != "WAITING_PURCHASE" {
		return domain.Payment{}, domain.ErrInvalidState
	}
	now := s.now().UTC()
	id := uuid.NewString()
	tx, err := s.gateway.CreateTransaction(ctx, id, id, summary.RemainingAmountVND, "VND")
	if err != nil {
		return domain.Payment{}, domain.ErrDependency
	}
	provider := "momo"
	if strings.HasPrefix(tx.Reference, "mock-") {
		provider = "mock"
	}
	p := domain.Payment{ID: id, OrderID: input.OrderID, Type: domain.TypeRemainingBalance, AmountVND: summary.RemainingAmountVND, Currency: domain.CurrencyVND, Status: domain.StatusPending, PaymentURL: tx.HostedURL, ProviderReference: tx.Reference, Provider: provider, ProviderRequestID: id, CreatedAt: now, UpdatedAt: now}
	if err = repo.Create(ctx, p); err != nil {
		return domain.Payment{}, err
	}
	return p, nil
}

func (s *Service) Financial(ctx context.Context, orderID, token string) (FinancialSummary, error) {
	auth, ok := s.orders.(ports.OrderRefundAuthorizer)
	if !ok {
		return FinancialSummary{}, domain.ErrDependency
	}
	if err := auth.AuthorizeRefund(ctx, orderID, token); err != nil {
		return FinancialSummary{}, err
	}
	return s.financial(ctx, orderID)
}
func (s *Service) financial(ctx context.Context, orderID string) (FinancialSummary, error) {
	if _, err := uuid.Parse(orderID); err != nil {
		return FinancialSummary{}, domain.ErrInvalidInput
	}
	repo, ok := s.repository.(ports.ExtendedRepository)
	if !ok {
		return FinancialSummary{}, domain.ErrDependency
	}
	ps, rs, err := repo.ListByOrder(ctx, orderID)
	if err != nil {
		return FinancialSummary{}, err
	}
	out := FinancialSummary{OrderID: orderID, Payments: ps, Refunds: rs}
	for _, p := range ps {
		if p.Status == domain.StatusSucceeded {
			out.PaidVND += p.AmountVND
		}
	}
	for _, r := range rs {
		if r.Status == domain.StatusSucceeded {
			out.RefundedVND += r.AmountVND
		}
	}
	out.NetPaidVND = out.PaidVND - out.RefundedVND
	return out, nil
}

func (s *Service) ApplyProviderResult(ctx context.Context, paymentID, requestID, transID string, resultCode int, message string) (domain.Payment, error) {
	repo, ok := s.repository.(ports.ExtendedRepository)
	if !ok {
		return domain.Payment{}, domain.ErrDependency
	}
	p, err := repo.FindByID(ctx, paymentID)
	if err != nil {
		return domain.Payment{}, err
	}
	if p.ProviderRequestID != requestID {
		return domain.Payment{}, domain.ErrInvalidInput
	}
	success := resultCode == 0
	var outbox ports.OutboxEvent
	if success {
		outbox, err = makeSucceededEvent(p, s.now().UTC())
		if err != nil {
			return domain.Payment{}, err
		}
	}
	result, _, err := repo.CompleteProviderResult(ctx, paymentID, transID, resultCode, message, success, outbox)
	return result, err
}

func makeSucceededEvent(payment domain.Payment, now time.Time) (ports.OutboxEvent, error) {
	if payment.Type == domain.TypeDeposit {
		return makeDepositSucceededEvent(payment, now)
	}
	data, _ := json.Marshal(depositSucceededData{PaymentID: payment.ID, OrderID: payment.OrderID, AmountVND: payment.AmountVND, Currency: payment.Currency})
	eventID := uuid.NewString()
	payload, err := json.Marshal(eventEnvelope{EventID: eventID, EventType: "payment.remaining_balance_succeeded.v1", AggregateID: payment.OrderID, Producer: "payment-service", OccurredAt: now, Data: data})
	return ports.OutboxEvent{ID: eventID, AggregateID: payment.OrderID, EventType: "payment.remaining_balance_succeeded.v1", Payload: payload, CreatedAt: now}, err
}

func (s *Service) ProcessCallback(ctx context.Context, providerReference, eventID, status string) (domain.Payment, error) {
	providerReference, eventID, status = strings.TrimSpace(providerReference), strings.TrimSpace(eventID), strings.ToUpper(strings.TrimSpace(status))
	if providerReference == "" || eventID == "" || status != "SUCCEEDED" {
		return domain.Payment{}, domain.ErrInvalidInput
	}
	lookup, ok := s.repository.(ports.PaymentLookup)
	if !ok {
		return domain.Payment{}, domain.ErrDependency
	}
	payment, err := lookup.FindByProviderReference(ctx, providerReference)
	if err != nil {
		return domain.Payment{}, err
	}
	outbox, err := makeDepositSucceededEvent(payment, s.now().UTC())
	if err != nil {
		return domain.Payment{}, err
	}
	callbacks, ok := s.repository.(ports.CallbackRepository)
	if !ok {
		return domain.Payment{}, domain.ErrDependency
	}
	result, _, err := callbacks.SucceedCallback(ctx, payment.ID, eventID, outbox)
	return result, err
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
