package application

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/example/cross-border-logistics/internal/order/domain"
	"github.com/example/cross-border-logistics/internal/order/ports"
	"github.com/google/uuid"
)

const quotationStatusPending = "PENDING_CONFIRMATION"

type CreateInput struct {
	QuotationID, CustomerID, DeliveryAddress string
}

type PaymentSummary struct {
	OrderID            string             `json:"orderId"`
	DepositAmountVND   int64              `json:"depositAmountVnd"`
	RemainingAmountVND int64              `json:"remainingAmountVnd"`
	Status             domain.OrderStatus `json:"status"`
}

type eventEnvelope struct {
	EventID     string          `json:"eventId"`
	EventType   string          `json:"eventType"`
	AggregateID string          `json:"aggregateId"`
	Producer    string          `json:"producer"`
	OccurredAt  time.Time       `json:"occurredAt"`
	Data        json.RawMessage `json:"data"`
}

type orderCreatedData struct {
	OrderID          string             `json:"orderId"`
	CustomerID       string             `json:"customerId"`
	QuotationID      string             `json:"quotationId"`
	TotalAmountVND   int64              `json:"totalAmountVnd"`
	DepositAmountVND int64              `json:"depositAmountVnd"`
	Status           domain.OrderStatus `json:"status"`
}

type Service struct {
	repository ports.OrderRepository
	quotations ports.QuotationReader
	now        func() time.Time
}

func NewService(repository ports.OrderRepository, quotations ports.QuotationReader) *Service {
	return &Service{repository: repository, quotations: quotations, now: time.Now}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (domain.Order, error) {
	input.QuotationID = strings.TrimSpace(input.QuotationID)
	input.CustomerID = strings.TrimSpace(input.CustomerID)
	input.DeliveryAddress = strings.TrimSpace(input.DeliveryAddress)
	if _, err := uuid.Parse(input.QuotationID); err != nil || input.CustomerID == "" || input.DeliveryAddress == "" {
		return domain.Order{}, domain.ErrInvalidInput
	}

	quotation, err := s.quotations.GetQuotation(ctx, input.QuotationID)
	if err != nil {
		return domain.Order{}, err
	}
	if quotation.CustomerID != input.CustomerID {
		return domain.Order{}, domain.ErrCustomerMismatch
	}
	if quotation.Status != quotationStatusPending || quotation.Quantity <= 0 || quotation.TotalAmountVND <= 0 {
		return domain.Order{}, domain.ErrInvalidQuotation
	}

	now := s.now().UTC()
	orderID := uuid.NewString()
	deposit := quotation.TotalAmountVND/100*70 + quotation.TotalAmountVND%100*70/100
	order := domain.Order{
		ID: orderID, CustomerID: input.CustomerID, QuotationID: quotation.QuotationID,
		DeliveryAddress: input.DeliveryAddress, TotalAmountVND: quotation.TotalAmountVND,
		DepositAmountVND: deposit, RemainingAmountVND: quotation.TotalAmountVND - deposit,
		Status: domain.StatusWaitingDeposit, CreatedAt: now, UpdatedAt: now,
		Items: []domain.OrderItem{{ID: uuid.NewString(), OrderID: orderID, ProductName: quotation.ProductName,
			ProductURL: quotation.ProductURL, Quantity: quotation.Quantity,
			UnitPriceVND: quotation.TotalAmountVND / int64(quotation.Quantity), TotalPriceVND: quotation.TotalAmountVND, CreatedAt: now}},
	}
	tracking := domain.TrackingEvent{ID: uuid.NewString(), OrderID: orderID, Status: order.Status,
		Description: domain.InitialTrackingDescription, Source: "order-service", OccurredAt: now, CreatedAt: now}
	outbox, err := makeOrderCreatedEvent(order, now)
	if err != nil {
		return domain.Order{}, err
	}
	if err := s.repository.Create(ctx, order, tracking, outbox); err != nil {
		return domain.Order{}, err
	}
	return order, nil
}

func (s *Service) Get(ctx context.Context, id string) (domain.Order, error) {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return domain.Order{}, domain.ErrInvalidInput
	}
	return s.repository.FindByID(ctx, id)
}

func (s *Service) Timeline(ctx context.Context, id string) ([]domain.TrackingEvent, error) {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return nil, domain.ErrInvalidInput
	}
	return s.repository.FindTimeline(ctx, id)
}

func (s *Service) GetPaymentSummary(ctx context.Context, id string) (PaymentSummary, error) {
	order, err := s.Get(ctx, id)
	if err != nil {
		return PaymentSummary{}, err
	}
	return PaymentSummary{OrderID: order.ID, DepositAmountVND: order.DepositAmountVND, RemainingAmountVND: order.RemainingAmountVND, Status: order.Status}, nil
}

func makeOrderCreatedEvent(order domain.Order, now time.Time) (ports.OutboxEvent, error) {
	data, err := json.Marshal(orderCreatedData{OrderID: order.ID, CustomerID: order.CustomerID, QuotationID: order.QuotationID, TotalAmountVND: order.TotalAmountVND, DepositAmountVND: order.DepositAmountVND, Status: order.Status})
	if err != nil {
		return ports.OutboxEvent{}, err
	}
	eventID := uuid.NewString()
	payload, err := json.Marshal(eventEnvelope{EventID: eventID, EventType: "order.created.v1", AggregateID: order.ID, Producer: "order-service", OccurredAt: now, Data: data})
	if err != nil {
		return ports.OutboxEvent{}, err
	}
	return ports.OutboxEvent{ID: eventID, AggregateID: order.ID, EventType: "order.created.v1", Payload: payload, CreatedAt: now}, nil
}
