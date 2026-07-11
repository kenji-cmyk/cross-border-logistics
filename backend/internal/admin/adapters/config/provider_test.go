package config_test

import (
	"context"
	"strings"
	"testing"
	"time"

	adminconfig "github.com/example/cross-border-logistics/internal/admin/adapters/config"
)

func lookup(values map[string]string) adminconfig.LookupFunc {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}

func TestLoadDefaults(t *testing.T) {
	provider, err := adminconfig.Load(lookup(nil), time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	rates, err := provider.GetSystemRates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rates.ServiceFeePercent != 5 || rates.EstimatedShippingFeeVND != 120000 || rates.DepositPercent != 70 {
		t.Fatalf("unexpected defaults: %+v", rates)
	}
	if got := strings.Join(rates.SupportedCurrencies, ","); got != "USD,CNY,JPY,KRW" {
		t.Fatalf("currencies = %s", got)
	}
	for currency, expected := range map[string]int64{"USD": 26000, "CNY": 3600, "JPY": 175, "KRW": 19} {
		if rates.ExchangeRates[currency] != expected {
			t.Fatalf("rate %s = %d", currency, rates.ExchangeRates[currency])
		}
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name, variable, value, message string
	}{
		{"service fee below zero", "ADMIN_SERVICE_FEE_PERCENT", "-1", "service fee percent"},
		{"service fee above 100", "ADMIN_SERVICE_FEE_PERCENT", "101", "service fee percent"},
		{"deposit below zero", "ADMIN_DEPOSIT_PERCENT", "-1", "deposit percent"},
		{"deposit above 100", "ADMIN_DEPOSIT_PERCENT", "101", "deposit percent"},
		{"negative shipping fee", "ADMIN_ESTIMATED_SHIPPING_FEE_VND", "-1", "shipping fee"},
		{"zero exchange rate", "ADMIN_EXCHANGE_RATE_USD", "0", "exchange rate"},
		{"negative exchange rate", "ADMIN_EXCHANGE_RATE_USD", "-1", "exchange rate"},
		{"empty currencies", "ADMIN_SUPPORTED_CURRENCIES", "  ", "supported currencies"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := adminconfig.Load(lookup(map[string]string{test.variable: test.value}), time.Now())
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), test.message) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestLoadNormalizesTrimsAndPreservesDeterministicOrder(t *testing.T) {
	provider, err := adminconfig.Load(lookup(map[string]string{
		"ADMIN_SUPPORTED_CURRENCIES": " krw, usd , cny, krw ",
	}), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	rates, _ := provider.GetSystemRates(context.Background())
	if got := strings.Join(rates.SupportedCurrencies, ","); got != "KRW,USD,CNY" {
		t.Fatalf("currencies = %s", got)
	}
	if rates.ExchangeRates["KRW"] != 19 || rates.ExchangeRates["USD"] != 26000 {
		t.Fatalf("rates = %#v", rates.ExchangeRates)
	}
}

func TestLoadRejectsMissingRateForSupportedCurrency(t *testing.T) {
	_, err := adminconfig.Load(lookup(map[string]string{"ADMIN_SUPPORTED_CURRENCIES": "USD,EUR"}), time.Now())
	if err == nil || !strings.Contains(err.Error(), "EUR") {
		t.Fatalf("error = %v", err)
	}
}
