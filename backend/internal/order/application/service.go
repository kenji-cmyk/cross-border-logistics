package application

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/example/cross-border-logistics/internal/order/domain"
	"github.com/example/cross-border-logistics/internal/order/ports"
	sharedevent "github.com/example/cross-border-logistics/pkg/event"
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

type WarehouseSummary struct {
	OrderID    string             `json:"orderId"`
	CustomerID string             `json:"customerId"`
	Status     domain.OrderStatus `json:"status"`
	CreatedAt  time.Time          `json:"createdAt"`
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
	if _, err := uuid.Parse(input.QuotationID); err != nil || input.DeliveryAddress == "" {
		return domain.Order{}, domain.ErrInvalidInput
	}

	quotation, err := s.quotations.GetQuotation(ctx, input.QuotationID)
	if err != nil {
		return domain.Order{}, err
	}
	if input.CustomerID != "" && quotation.CustomerID != input.CustomerID {
		return domain.Order{}, domain.ErrCustomerMismatch
	}
	_, canConfirm := s.quotations.(ports.QuotationConfirmer)
	if (quotation.Status != quotationStatusPending && !(canConfirm && quotation.Status == "CONFIRMED")) || quotation.Quantity <= 0 || quotation.TotalAmountVND <= 0 {
		return domain.Order{}, domain.ErrInvalidQuotation
	}

	now := s.now().UTC()
	orderID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("order:"+quotation.QuotationID)).String()
	if confirmer, ok := s.quotations.(ports.QuotationConfirmer); ok {
		confirmed, err := confirmer.ConfirmQuotation(ctx, quotation.QuotationID, orderID)
		if err != nil {
			return domain.Order{}, err
		}
		quotation = confirmed
	}
	if input.CustomerID == "" {
		input.CustomerID = quotation.CustomerID
	}
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

func (s *Service) GetWarehouseSummary(ctx context.Context, id string) (WarehouseSummary, error) {
	order, err := s.Get(ctx, id)
	if err != nil {
		return WarehouseSummary{}, err
	}
	return WarehouseSummary{OrderID: order.ID, CustomerID: order.CustomerID, Status: order.Status, CreatedAt: order.CreatedAt}, nil
}

func (s *Service) HandlePackageReceived(ctx context.Context, envelope sharedevent.Envelope) (bool, error) {
	if envelope.EventType != sharedevent.PackageReceived || envelope.Producer != "warehouse-service" || envelope.EventID == uuid.Nil || envelope.AggregateID == uuid.Nil || envelope.OccurredAt.IsZero() {
		return false, domain.ErrInvalidInput
	}
	var data sharedevent.PackageReceivedData
	if err := json.Unmarshal(envelope.Data, &data); err != nil || data.PackageID == uuid.Nil || data.OrderID == uuid.Nil || data.OrderID != envelope.AggregateID || strings.TrimSpace(data.SourceTrackingNumber) == "" || strings.TrimSpace(data.WarehouseCode) == "" || data.WeightKg <= 0 || data.LengthCm <= 0 || data.WidthCm <= 0 || data.HeightCm <= 0 {
		return false, domain.ErrInvalidInput
	}
	now := s.now().UTC()
	description := domain.PackageReceivedTrackingDescription
	statusEvent, err := sharedevent.New(sharedevent.OrderStatusChanged, data.OrderID, "order-service", now, sharedevent.OrderStatusChangedData{OrderID: data.OrderID, PreviousStatus: string(domain.StatusWaitingPurchase), CurrentStatus: string(domain.StatusArrivedForeignWarehouse), Description: description})
	if err != nil {
		return false, err
	}
	payload, err := statusEvent.Marshal()
	if err != nil {
		return false, err
	}
	tracking := domain.TrackingEvent{ID: uuid.NewString(), OrderID: data.OrderID.String(), Status: domain.StatusArrivedForeignWarehouse, Description: description, Source: "warehouse-service", OccurredAt: envelope.OccurredAt.UTC(), CreatedAt: now}
	return s.repository.ProcessPackageReceived(ctx, ports.ProcessPackageReceived{EventID: envelope.EventID.String(), EventType: envelope.EventType, OrderID: data.OrderID.String(), ProcessedAt: now, Tracking: tracking, Outbox: ports.OutboxEvent{ID: statusEvent.EventID.String(), AggregateID: data.OrderID.String(), EventType: statusEvent.EventType, Payload: payload, CreatedAt: now}})
}

func (s *Service) HandlePaymentDepositSucceeded(ctx context.Context, envelope sharedevent.Envelope) (bool, error) {
	if envelope.EventType != sharedevent.PaymentDepositSucceeded || envelope.Producer != "payment-service" || envelope.EventID == uuid.Nil || envelope.AggregateID == uuid.Nil {
		return false, domain.ErrInvalidInput
	}
	var data sharedevent.PaymentDepositSucceededData
	if err := json.Unmarshal(envelope.Data, &data); err != nil || data.OrderID == uuid.Nil || data.PaymentID == uuid.Nil || data.AmountVND <= 0 || data.Currency != "VND" || data.OrderID != envelope.AggregateID {
		return false, domain.ErrInvalidInput
	}
	now := s.now().UTC()
	description := domain.DepositSucceededTrackingDescription
	statusEvent, err := sharedevent.New(sharedevent.OrderStatusChanged, data.OrderID, "order-service", now, sharedevent.OrderStatusChangedData{
		OrderID: data.OrderID, PreviousStatus: string(domain.StatusWaitingDeposit), CurrentStatus: string(domain.StatusWaitingPurchase), Description: description,
	})
	if err != nil {
		return false, err
	}
	payload, err := statusEvent.Marshal()
	if err != nil {
		return false, err
	}
	tracking := domain.TrackingEvent{ID: uuid.NewString(), OrderID: data.OrderID.String(), Status: domain.StatusWaitingPurchase, Description: description, Source: "payment-service", OccurredAt: now, CreatedAt: now}
	return s.repository.ProcessPaymentSucceeded(ctx, ports.ProcessPaymentSucceeded{
		EventID: envelope.EventID.String(), EventType: envelope.EventType, OrderID: data.OrderID.String(), ProcessedAt: now, Tracking: tracking,
		AmountVND: data.AmountVND,
		Outbox:    ports.OutboxEvent{ID: statusEvent.EventID.String(), AggregateID: data.OrderID.String(), EventType: statusEvent.EventType, Payload: payload, CreatedAt: now},
	})
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
