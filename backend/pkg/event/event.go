package event

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	OrderCreated              = "order.created.v1"
	PaymentDepositSucceeded   = "payment.deposit_succeeded.v1"
	PaymentRemainingSucceeded = "payment.remaining_balance_succeeded.v1"
	PaymentRefundSucceeded    = "payment.refund_succeeded.v1"
	OrderStatusChanged        = "order.status_changed.v1"
	PackageReceived           = "package.received.v1"
)

type Envelope struct {
	EventID     uuid.UUID       `json:"eventId"`
	EventType   string          `json:"eventType"`
	AggregateID uuid.UUID       `json:"aggregateId"`
	Producer    string          `json:"producer"`
	OccurredAt  time.Time       `json:"occurredAt"`
	Data        json.RawMessage `json:"data"`
}
type PaymentRefundSucceededData struct {
	RefundID         uuid.UUID `json:"refundId"`
	PaymentID        uuid.UUID `json:"paymentId"`
	OrderID          uuid.UUID `json:"orderId"`
	AmountVND        int64     `json:"amountVnd"`
	TotalRefundedVND int64     `json:"totalRefundedVnd"`
}

func New(eventType string, aggregateID uuid.UUID, producer string, occurredAt time.Time, data any) (Envelope, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Envelope{}, fmt.Errorf("marshal event data: %w", err)
	}
	return Envelope{EventID: uuid.New(), EventType: eventType, AggregateID: aggregateID, Producer: producer, OccurredAt: occurredAt.UTC(), Data: raw}, nil
}

func (e Envelope) Marshal() ([]byte, error) {
	payload, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("marshal event envelope: %w", err)
	}
	return payload, nil
}

type PaymentDepositSucceededData struct {
	PaymentID uuid.UUID `json:"paymentId"`
	OrderID   uuid.UUID `json:"orderId"`
	AmountVND int64     `json:"amountVnd"`
	Currency  string    `json:"currency"`
}

type OrderStatusChangedData struct {
	OrderID        uuid.UUID `json:"orderId"`
	PreviousStatus string    `json:"previousStatus"`
	CurrentStatus  string    `json:"currentStatus"`
	Description    string    `json:"description"`
}

type PackageReceivedData struct {
	PackageID            uuid.UUID `json:"packageId"`
	OrderID              uuid.UUID `json:"orderId"`
	SourceTrackingNumber string    `json:"sourceTrackingNumber"`
	WarehouseCode        string    `json:"warehouseCode"`
	WeightKg             float64   `json:"weightKg"`
	LengthCm             float64   `json:"lengthCm"`
	WidthCm              float64   `json:"widthCm"`
	HeightCm             float64   `json:"heightCm"`
}
