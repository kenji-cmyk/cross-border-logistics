package domain

import (
	"fmt"
	"strings"
	"time"
)

type SystemRates struct {
	ServiceFeePercent       int
	EstimatedShippingFeeVND int64
	DepositPercent          int
	SupportedCurrencies     []string
	ExchangeRates           map[string]int64
	EffectiveAt             time.Time
}

func (r SystemRates) Validate() error {
	if r.ServiceFeePercent < 0 || r.ServiceFeePercent > 100 {
		return fmt.Errorf("service fee percent must be between 0 and 100")
	}
	if r.DepositPercent < 0 || r.DepositPercent > 100 {
		return fmt.Errorf("deposit percent must be between 0 and 100")
	}
	if r.EstimatedShippingFeeVND < 0 {
		return fmt.Errorf("estimated shipping fee VND must not be negative")
	}
	if len(r.SupportedCurrencies) == 0 {
		return fmt.Errorf("supported currencies must not be empty")
	}
	for _, currency := range r.SupportedCurrencies {
		code := strings.ToUpper(strings.TrimSpace(currency))
		if code == "" {
			return fmt.Errorf("supported currency must not be empty")
		}
		rate, ok := r.ExchangeRates[code]
		if !ok {
			return fmt.Errorf("exchange rate for supported currency %s is missing", code)
		}
		if rate <= 0 {
			return fmt.Errorf("exchange rate for %s must be greater than zero", code)
		}
	}
	return nil
}
