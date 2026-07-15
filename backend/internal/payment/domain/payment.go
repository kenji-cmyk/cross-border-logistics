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
	ErrPaymentNotFound     = errors.New("payment not found")
	ErrPaymentConflict     = errors.New("payment already exists for order and type")
	ErrInvalidInput        = errors.New("invalid payment input")
	ErrInvalidState        = errors.New("invalid payment state")
	ErrOrderNotFound       = errors.New("order not found")
	ErrDependency          = errors.New("order service dependency failed")
	ErrCheckoutUnavailable = errors.New("hosted checkout is unavailable")
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
	CreatedAt         time.Time     `json:"createdAt"`
	UpdatedAt         time.Time     `json:"updatedAt"`
}

func CanTransition(from, to PaymentStatus) bool {
	return from == StatusPending && to == StatusSucceeded
}
