package http_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	paymenthttp "github.com/example/cross-border-logistics/internal/payment/adapters/http"
	"github.com/example/cross-border-logistics/internal/payment/application"
	"github.com/example/cross-border-logistics/internal/payment/domain"
	"github.com/example/cross-border-logistics/pkg/httpx"
)

const paymentID = "9f42fc31-e997-4b6f-a742-981ca145bacc"

type fakeService struct{ err error }

func (f fakeService) CreateDeposit(context.Context, application.CreateDepositInput) (domain.Payment, error) {
	return domain.Payment{ID: paymentID, OrderID: "46ab7a1a-bab7-4a46-b9f9-d7572a284895", Type: domain.TypeDeposit, AmountVND: 700, Currency: domain.CurrencyVND, Status: domain.StatusPending}, f.err
}
func (f fakeService) Get(context.Context, string) (domain.Payment, error) {
	return domain.Payment{ID: paymentID, Status: domain.StatusPending}, f.err
}
func (f fakeService) MarkSucceeded(context.Context, string) (domain.Payment, error) {
	return domain.Payment{ID: paymentID, Status: domain.StatusSucceeded}, f.err
}

func handler(service fakeService) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return httpx.RequestIDMiddleware(paymenthttp.New(service, logger, "payment-service"))
}

func TestPaymentEndpoints(t *testing.T) {
	tests := []struct {
		method, path, body string
		status             int
		contains           string
	}{
		{http.MethodPost, "/api/v1/payments/deposits", `{"orderId":"46ab7a1a-bab7-4a46-b9f9-d7572a284895"}`, http.StatusCreated, `"status":"PENDING"`},
		{http.MethodGet, "/api/v1/payments/" + paymentID, "", http.StatusOK, `"paymentId":"` + paymentID + `"`},
		{http.MethodPost, "/api/v1/payments/" + paymentID + "/mock-success", "", http.StatusOK, `"status":"SUCCEEDED"`},
	}
	for _, tt := range tests {
		response := httptest.NewRecorder()
		handler(fakeService{}).ServeHTTP(response, httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body)))
		if response.Code != tt.status || !strings.Contains(response.Body.String(), tt.contains) {
			t.Fatalf("%s %s status=%d body=%s", tt.method, tt.path, response.Code, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), `"requestId"`) {
			t.Fatalf("missing response meta: %s", response.Body.String())
		}
	}
}

func TestPaymentHandlerErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"validation", domain.ErrInvalidInput, http.StatusBadRequest, "VALIDATION_ERROR"},
		{"not found", domain.ErrPaymentNotFound, http.StatusNotFound, "NOT_FOUND"},
		{"duplicate", domain.ErrPaymentConflict, http.StatusConflict, "CONFLICT"},
		{"invalid state", domain.ErrInvalidState, http.StatusConflict, "INVALID_STATE"},
		{"dependency", domain.ErrDependency, http.StatusBadGateway, "DEPENDENCY_ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler(fakeService{err: tt.err}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/payments/"+paymentID, nil))
			if response.Code != tt.status || !strings.Contains(response.Body.String(), tt.code) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestCreateDepositRejectsMalformedBody(t *testing.T) {
	response := httptest.NewRecorder()
	handler(fakeService{}).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/payments/deposits", strings.NewReader(`{`)))
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "VALIDATION_ERROR") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
