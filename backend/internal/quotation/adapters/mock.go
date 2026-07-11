package adapters

import (
	"context"
	"strings"

	"github.com/example/cross-border-logistics/internal/quotation/domain"
)

type MockExchangeRates struct{}

func (MockExchangeRates) Rate(_ context.Context, currency string) (int64, error) {
	rates := map[string]int64{"USD": 26000, "CNY": 3600, "JPY": 175, "KRW": 19}
	rate, ok := rates[strings.ToUpper(currency)]
	if !ok {
		return 0, domain.ErrUnsupportedCurrency
	}
	return rate, nil
}

type MockRestrictionChecker struct{}

func (MockRestrictionChecker) IsRestricted(_ context.Context, name, url string) bool {
	value := strings.ToLower(name + " " + url)
	for _, keyword := range []string{"weapon", "gun", "explosive", "battery-liquid", "dangerous-chemical"} {
		if strings.Contains(value, keyword) {
			return true
		}
	}
	return false
}
