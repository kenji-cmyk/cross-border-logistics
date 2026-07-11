package domain

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

type Status string

const (
	StatusPendingConfirmation Status = "PENDING_CONFIRMATION"
	StatusConfirmed           Status = "CONFIRMED"
	StatusExpired             Status = "EXPIRED"
	StatusRejected            Status = "REJECTED"
	SourcePriceScale                 = int64(1_000_000)
	EstimatedShippingFeeVND          = int64(120_000)
)

var (
	ErrQuotationNotFound     = errors.New("quotation not found")
	ErrRestrictedProduct     = errors.New("product is restricted")
	ErrUnsupportedCurrency   = errors.New("unsupported currency")
	ErrInvalidQuotationInput = errors.New("invalid quotation input")
)

// Quotation stores source price as a fixed-point value with six decimal places.
// This keeps input and VND calculations exact without floating-point arithmetic.
type Quotation struct {
	ID                      string
	CustomerID              string
	ProductURL              string
	ProductName             string
	SourcePriceMicros       int64
	Currency                string
	Quantity                int
	ExchangeRate            int64
	ProductAmountVND        int64
	ServiceFeeVND           int64
	EstimatedShippingFeeVND int64
	TotalAmountVND          int64
	Status                  Status
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

func ParseSourcePrice(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "-") {
		return 0, ErrInvalidQuotationInput
	}
	parts := strings.Split(raw, ".")
	if len(parts) > 2 || len(parts[0]) == 0 {
		return 0, ErrInvalidQuotationInput
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 6 {
		return 0, ErrInvalidQuotationInput
	}
	for len(fraction) < 6 {
		fraction += "0"
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, ErrInvalidQuotationInput
	}
	frac := int64(0)
	if fraction != "" {
		frac, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, ErrInvalidQuotationInput
		}
	}
	if whole > (int64(^uint64(0)>>1)-frac)/SourcePriceScale {
		return 0, ErrInvalidQuotationInput
	}
	value := whole*SourcePriceScale + frac
	if value <= 0 {
		return 0, ErrInvalidQuotationInput
	}
	return value, nil
}

func FormatSourcePrice(value int64) string {
	whole, fraction := value/SourcePriceScale, value%SourcePriceScale
	if fraction == 0 {
		return strconv.FormatInt(whole, 10)
	}
	return strconv.FormatInt(whole, 10) + "." + strings.TrimRight(fmt.Sprintf("%06d", fraction), "0")
}

func Calculate(sourcePriceMicros int64, quantity int, exchangeRate int64) (int64, int64, int64, error) {
	if sourcePriceMicros <= 0 || quantity <= 0 || exchangeRate <= 0 {
		return 0, 0, 0, ErrInvalidQuotationInput
	}
	product := big.NewInt(sourcePriceMicros)
	product.Mul(product, big.NewInt(int64(quantity)))
	product.Mul(product, big.NewInt(exchangeRate))
	product.Add(product, big.NewInt(SourcePriceScale/2))
	product.Quo(product, big.NewInt(SourcePriceScale))
	if !product.IsInt64() {
		return 0, 0, 0, ErrInvalidQuotationInput
	}
	amount := product.Int64()
	feeBig := big.NewInt(amount)
	feeBig.Mul(feeBig, big.NewInt(5))
	feeBig.Add(feeBig, big.NewInt(50))
	feeBig.Quo(feeBig, big.NewInt(100))
	if !feeBig.IsInt64() || amount > int64(^uint64(0)>>1)-feeBig.Int64()-EstimatedShippingFeeVND {
		return 0, 0, 0, ErrInvalidQuotationInput
	}
	fee := feeBig.Int64()
	return amount, fee, amount + fee + EstimatedShippingFeeVND, nil
}
