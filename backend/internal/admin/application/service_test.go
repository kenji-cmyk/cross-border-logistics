package application_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/kenji-cmyk/cross-border-logistics/internal/admin/application"
	"github.com/kenji-cmyk/cross-border-logistics/internal/admin/domain"
)

type fakeProvider struct {
	rates domain.SystemRates
	err   error
}

func (f fakeProvider) GetSystemRates(context.Context) (domain.SystemRates, error) {
	return f.rates, f.err
}

func TestGetSystemRatesMapsAndNormalizesProviderData(t *testing.T) {
	effectiveAt := time.Date(2026, 7, 11, 4, 30, 0, 0, time.FixedZone("test", 7*60*60))
	provider := fakeProvider{rates: domain.SystemRates{
		ServiceFeePercent: 5, EstimatedShippingFeeVND: 120000, DepositPercent: 70,
		SupportedCurrencies: []string{" usd ", "CNY", "usd", "jpy", "KRW"},
		ExchangeRates:       map[string]int64{"usd": 26000, "CNY": 3600, "jpy": 175, "KRW": 19},
		EffectiveAt:         effectiveAt,
	}}
	result, err := application.NewGetSystemRates(provider).Execute(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.ServiceFeePercent != 5 || result.EstimatedShippingFeeVND != 120000 || result.DepositPercent != 70 {
		t.Fatalf("result = %+v", result)
	}
	if !reflect.DeepEqual(result.SupportedCurrencies, []string{"CNY", "JPY", "KRW", "USD"}) {
		t.Fatalf("currencies = %#v", result.SupportedCurrencies)
	}
	if result.ExchangeRates["USD"] != 26000 || result.ExchangeRates["KRW"] != 19 {
		t.Fatalf("rates = %#v", result.ExchangeRates)
	}
	if !result.EffectiveAt.Equal(effectiveAt) || result.EffectiveAt.Location() != time.UTC {
		t.Fatalf("effectiveAt = %s", result.EffectiveAt)
	}
	if _, exists := reflect.TypeOf(result).FieldByName("DatabaseURL"); exists {
		t.Fatal("application DTO exposes internal configuration")
	}
}

func TestGetSystemRatesPropagatesProviderError(t *testing.T) {
	want := errors.New("provider failed")
	_, err := application.NewGetSystemRates(fakeProvider{err: want}).Execute(context.Background())
	if !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
}
