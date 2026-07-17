package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	adminhttp "github.com/kenji-cmyk/cross-border-logistics/internal/admin/adapters/http"
	"github.com/kenji-cmyk/cross-border-logistics/internal/admin/application"
	"github.com/kenji-cmyk/cross-border-logistics/pkg/httpx"
)

type fakeUseCase struct {
	rates application.SystemRates
	err   error
}

func (f fakeUseCase) Execute(context.Context) (application.SystemRates, error) { return f.rates, f.err }

func testHandler(useCase fakeUseCase) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return httpx.RequestIDMiddleware(adminhttp.New(useCase, logger, "admin-service"))
}

func sampleRates() application.SystemRates {
	return application.SystemRates{
		ServiceFeePercent: 5, EstimatedShippingFeeVND: 120000, DepositPercent: 70,
		SupportedCurrencies: []string{"CNY", "JPY", "KRW", "USD"},
		ExchangeRates:       map[string]int64{"USD": 26000, "CNY": 3600, "JPY": 175, "KRW": 19},
		EffectiveAt:         time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC),
	}
}

func TestRatesReturnsSuccessEnvelopeAndPropagatesRequestID(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/rates", nil)
	request.Header.Set("X-Request-ID", "phase-7-test-request")
	response := httptest.NewRecorder()
	testHandler(fakeUseCase{rates: sampleRates()}).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if response.Header().Get("X-Request-ID") != "phase-7-test-request" {
		t.Fatalf("header request ID = %q", response.Header().Get("X-Request-ID"))
	}
	var body struct {
		Data application.SystemRates `json:"data"`
		Meta httpx.Meta              `json:"meta"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Meta.RequestID != "phase-7-test-request" || body.Data.ServiceFeePercent != 5 || body.Data.EstimatedShippingFeeVND != 120000 || body.Data.DepositPercent != 70 {
		t.Fatalf("body = %+v", body)
	}
	for currency, rate := range map[string]int64{"USD": 26000, "CNY": 3600, "JPY": 175, "KRW": 19} {
		if body.Data.ExchangeRates[currency] != rate {
			t.Fatalf("rate %s = %d", currency, body.Data.ExchangeRates[currency])
		}
	}
}

func TestRatesSupportsTrailingSlash(t *testing.T) {
	response := httptest.NewRecorder()
	testHandler(fakeUseCase{rates: sampleRates()}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/admin/rates/", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestRatesProviderFailureReturnsInternalErrorEnvelope(t *testing.T) {
	response := httptest.NewRecorder()
	testHandler(fakeUseCase{err: errors.New("configuration unavailable")}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/admin/rates", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body httpx.ErrorResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "INTERNAL_ERROR" || body.Error.Message != "unable to load system rates" || body.Meta.RequestID == "" {
		t.Fatalf("body = %+v", body)
	}
}

func TestAdminRoutesAndUnsupportedMethod(t *testing.T) {
	handler := testHandler(fakeUseCase{rates: sampleRates()})
	for _, path := range []string{"/health", "/ready", "/api/v1/admin/rates"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s status=%d", path, response.Code)
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/admin/rates", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status=%d body=%s", response.Code, response.Body.String())
	}
}
