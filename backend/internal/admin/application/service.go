package application

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/kenji-cmyk/cross-border-logistics/internal/admin/ports"
)

type SystemRates struct {
	ServiceFeePercent       int              `json:"serviceFeePercent"`
	EstimatedShippingFeeVND int64            `json:"estimatedShippingFeeVnd"`
	DepositPercent          int              `json:"depositPercent"`
	SupportedCurrencies     []string         `json:"supportedCurrencies"`
	ExchangeRates           map[string]int64 `json:"exchangeRates"`
	EffectiveAt             time.Time        `json:"effectiveAt"`
}

type GetSystemRates struct{ provider ports.RatesProvider }

func NewGetSystemRates(provider ports.RatesProvider) *GetSystemRates {
	return &GetSystemRates{provider: provider}
}

func (uc *GetSystemRates) Execute(ctx context.Context) (SystemRates, error) {
	rates, err := uc.provider.GetSystemRates(ctx)
	if err != nil {
		return SystemRates{}, err
	}

	currencies := make([]string, 0, len(rates.SupportedCurrencies))
	seen := make(map[string]struct{}, len(rates.SupportedCurrencies))
	for _, currency := range rates.SupportedCurrencies {
		code := strings.ToUpper(strings.TrimSpace(currency))
		if _, exists := seen[code]; code == "" || exists {
			continue
		}
		seen[code] = struct{}{}
		currencies = append(currencies, code)
	}
	sort.Strings(currencies)

	exchangeRates := make(map[string]int64, len(rates.ExchangeRates))
	for currency, rate := range rates.ExchangeRates {
		exchangeRates[strings.ToUpper(strings.TrimSpace(currency))] = rate
	}

	return SystemRates{
		ServiceFeePercent:       rates.ServiceFeePercent,
		EstimatedShippingFeeVND: rates.EstimatedShippingFeeVND,
		DepositPercent:          rates.DepositPercent,
		SupportedCurrencies:     currencies,
		ExchangeRates:           exchangeRates,
		EffectiveAt:             rates.EffectiveAt.UTC(),
	}, nil
}
