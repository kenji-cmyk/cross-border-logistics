package adapters_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kenji-cmyk/cross-border-logistics/internal/quotation/adapters"
	"github.com/kenji-cmyk/cross-border-logistics/internal/quotation/domain"
)

func TestAdminHTTPExchangeRatesReadsAdminSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"exchangeRates":{"USD":26470,"CNY":3940}},"meta":{"requestId":"test"}}`))
	}))
	defer server.Close()

	provider := adapters.NewAdminHTTPExchangeRates(server.URL, time.Second)
	rate, err := provider.Rate(context.Background(), "usd")
	if err != nil || rate != 26470 {
		t.Fatalf("rate=%d err=%v", rate, err)
	}
	_, err = provider.Rate(context.Background(), "JPY")
	if !errors.Is(err, domain.ErrUnsupportedCurrency) {
		t.Fatalf("missing currency error = %v", err)
	}
}

func TestAdminHTTPExchangeRatesMapsProviderFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	provider := adapters.NewAdminHTTPExchangeRates(server.URL, time.Second)
	if _, err := provider.Rate(context.Background(), "USD"); !errors.Is(err, domain.ErrExchangeUnavailable) {
		t.Fatalf("error = %v", err)
	}
}
