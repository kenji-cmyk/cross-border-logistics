package domain

import (
	"errors"
	"time"
)

type PaymentType string

const (
	TypeDeposit          PaymentType = "DEPOSIT"
	TypeRemainingBalance PaymentType = "REMAINING_BALANCE"
	TypeRefund           PaymentType = "REFUND"
)

type PaymentStatus string

const (
	StatusPending   PaymentStatus = "PENDING"
	StatusSucceeded PaymentStatus = "SUCCEEDED"
	StatusFailed    PaymentStatus = "FAILED"
	StatusCancelled PaymentStatus = "CANCELLED"
	StatusRefunded  PaymentStatus = "REFUNDED"
)

type Currency string

const CurrencyVND Currency = "VND"

var (
	ErrPaymentNotFound = errors.New("payment not found")
	ErrPaymentConflict = errors.New("deposit payment already exists")
	ErrInvalidInput    = errors.New("invalid payment input")
	ErrInvalidState    = errors.New("invalid payment state")
	ErrOrderNotFound   = errors.New("order not found")
	ErrDependency      = errors.New("order service dependency failed")
)

type Payment struct {
	ID                string        `json:"paymentId"`
	OrderID           string        `json:"orderId"`
	Type              PaymentType   `json:"type"`
	AmountVND         int64         `json:"amountVnd"`
	Currency          Currency      `json:"currency"`
	Status            PaymentStatus `json:"status"`
	PaymentURL        string        `json:"paymentUrl"`
	ProviderReference string        `json:"providerReference"`
	Provider          string        `json:"provider"`
	ProviderRequestID string        `json:"providerRequestId"`
	ProviderTransID   string        `json:"providerTransactionId,omitempty"`
	ResultCode        *int          `json:"resultCode,omitempty"`
	ResultMessage     string        `json:"resultMessage,omitempty"`
	SucceededAt       *time.Time    `json:"succeededAt,omitempty"`
	FailedAt          *time.Time    `json:"failedAt,omitempty"`
	CreatedAt         time.Time     `json:"createdAt"`
	UpdatedAt         time.Time     `json:"updatedAt"`
}

type Refund struct {
	ID                string        `json:"refundId"`
	PaymentID         string        `json:"paymentId"`
	OrderID           string        `json:"orderId"`
	Provider          string        `json:"provider"`
	ProviderRequestID string        `json:"providerRequestId"`
	ProviderTransID   string        `json:"providerTransactionId,omitempty"`
	ResultMessage     string        `json:"resultMessage,omitempty"`
	AmountVND         int64         `json:"amountVnd"`
	Status            PaymentStatus `json:"status"`
	ResultCode        *int          `json:"resultCode,omitempty"`
	CreatedAt         time.Time     `json:"createdAt"`
	UpdatedAt         time.Time     `json:"updatedAt"`
	SucceededAt       *time.Time    `json:"succeededAt,omitempty"`
	FailedAt          *time.Time    `json:"failedAt,omitempty"`
}

func CanTransition(from, to PaymentStatus) bool {
	return from == StatusPending && (to == StatusSucceeded || to == StatusFailed || to == StatusCancelled)
}
