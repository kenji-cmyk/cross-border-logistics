package domain

import (
	"errors"
	"time"
)

type OrderStatus string

const (
	StatusWaitingDeposit                OrderStatus = "WAITING_DEPOSIT"
	StatusWaitingPurchase               OrderStatus = "WAITING_PURCHASE"
	StatusPurchased                     OrderStatus = "PURCHASED"
	StatusInTransitToForeignWarehouse   OrderStatus = "IN_TRANSIT_TO_FOREIGN_WAREHOUSE"
	StatusArrivedForeignWarehouse       OrderStatus = "ARRIVED_FOREIGN_WAREHOUSE"
	StatusPacked                        OrderStatus = "PACKED"
	StatusInTransitToDomesticWarehouse  OrderStatus = "IN_TRANSIT_TO_DOMESTIC_WAREHOUSE"
	StatusArrivedDomesticWarehouse      OrderStatus = "ARRIVED_DOMESTIC_WAREHOUSE"
	StatusWaitingRemainingPayment       OrderStatus = "WAITING_REMAINING_PAYMENT"
	StatusReadyForDomesticDelivery      OrderStatus = "READY_FOR_DOMESTIC_DELIVERY"
	StatusOutForDelivery                OrderStatus = "OUT_FOR_DELIVERY"
	StatusDelivered                     OrderStatus = "DELIVERED"
	StatusCancelled                     OrderStatus = "CANCELLED"
	InitialTrackingDescription                      = "Order is waiting for deposit payment"
	DepositSucceededTrackingDescription             = "Deposit payment received; order is waiting for purchase"
)

var (
	ErrOrderNotFound     = errors.New("order not found")
	ErrQuotationNotFound = errors.New("quotation not found")
	ErrQuotationConflict = errors.New("quotation already has an order")
	ErrInvalidInput      = errors.New("invalid order input")
	ErrInvalidQuotation  = errors.New("quotation is not eligible for order creation")
	ErrCustomerMismatch  = errors.New("quotation customer does not match")
	ErrInvalidTransition = errors.New("invalid order status transition")
	ErrDependency        = errors.New("quotation service dependency failed")
)

type Order struct {
	ID                 string      `json:"orderId"`
	CustomerID         string      `json:"customerId"`
	QuotationID        string      `json:"quotationId"`
	DeliveryAddress    string      `json:"deliveryAddress"`
	TotalAmountVND     int64       `json:"totalAmountVnd"`
	DepositAmountVND   int64       `json:"depositAmountVnd"`
	RemainingAmountVND int64       `json:"remainingAmountVnd"`
	Status             OrderStatus `json:"status"`
	CreatedAt          time.Time   `json:"createdAt"`
	UpdatedAt          time.Time   `json:"updatedAt"`
	Items              []OrderItem `json:"items"`
}

type OrderItem struct {
	ID            string    `json:"id"`
	OrderID       string    `json:"orderId"`
	ProductName   string    `json:"productName"`
	ProductURL    string    `json:"productUrl"`
	Quantity      int       `json:"quantity"`
	UnitPriceVND  int64     `json:"unitPriceVnd"`
	TotalPriceVND int64     `json:"totalPriceVnd"`
	CreatedAt     time.Time `json:"createdAt"`
}

type TrackingEvent struct {
	ID          string      `json:"id"`
	OrderID     string      `json:"orderId"`
	Status      OrderStatus `json:"status"`
	Description string      `json:"description"`
	Source      string      `json:"source"`
	OccurredAt  time.Time   `json:"occurredAt"`
	CreatedAt   time.Time   `json:"createdAt"`
}

func CanTransition(from, to OrderStatus) bool {
	if to == StatusCancelled {
		return from != StatusCancelled && from != StatusDelivered
	}
	allowed := map[OrderStatus][]OrderStatus{
		StatusWaitingDeposit:               {StatusWaitingPurchase},
		StatusWaitingPurchase:              {StatusPurchased, StatusArrivedForeignWarehouse},
		StatusPurchased:                    {StatusInTransitToForeignWarehouse},
		StatusInTransitToForeignWarehouse:  {StatusArrivedForeignWarehouse},
		StatusArrivedForeignWarehouse:      {StatusPacked},
		StatusPacked:                       {StatusInTransitToDomesticWarehouse},
		StatusInTransitToDomesticWarehouse: {StatusArrivedDomesticWarehouse},
		StatusArrivedDomesticWarehouse:     {StatusWaitingRemainingPayment},
		StatusWaitingRemainingPayment:      {StatusReadyForDomesticDelivery},
		StatusReadyForDomesticDelivery:     {StatusOutForDelivery},
		StatusOutForDelivery:               {StatusDelivered},
	}
	for _, candidate := range allowed[from] {
		if candidate == to {
			return true
		}
	}
	return false
}
