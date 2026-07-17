package http_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	orderhttp "github.com/kenji-cmyk/cross-border-logistics/internal/order/adapters/http"
	"github.com/kenji-cmyk/cross-border-logistics/internal/order/application"
	"github.com/kenji-cmyk/cross-border-logistics/internal/order/domain"
	"github.com/kenji-cmyk/cross-border-logistics/pkg/httpx"
)

const orderID = "46ab7a1a-bab7-4a46-b9f9-d7572a284895"

type fakeService struct{ err error }

func (f fakeService) Create(context.Context, application.CreateInput) (domain.Order, error) {
	return domain.Order{ID: orderID, QuotationID: orderID, CustomerID: "customer-001", DepositAmountVND: 700, RemainingAmountVND: 300, Status: domain.StatusWaitingDeposit, Items: []domain.OrderItem{}}, f.err
}
func (f fakeService) Get(context.Context, string) (domain.Order, error) {
	return domain.Order{ID: orderID, Status: domain.StatusWaitingDeposit, Items: []domain.OrderItem{}}, f.err
}
func (f fakeService) Timeline(context.Context, string) ([]domain.TrackingEvent, error) {
	return []domain.TrackingEvent{{OrderID: orderID, Status: domain.StatusWaitingDeposit}}, f.err
}
func (f fakeService) GetPaymentSummary(context.Context, string) (application.PaymentSummary, error) {
	return application.PaymentSummary{OrderID: orderID, DepositAmountVND: 700, RemainingAmountVND: 300, Status: domain.StatusWaitingDeposit}, f.err
}
func (f fakeService) GetWarehouseSummary(context.Context, string) (application.WarehouseSummary, error) {
	return application.WarehouseSummary{OrderID: orderID, CustomerID: "customer-001", Status: domain.StatusWaitingPurchase}, f.err
}

func handler(service fakeService) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return httpx.RequestIDMiddleware(orderhttp.New(service, logger, "order-service"))
}

func TestOrderEndpoints(t *testing.T) {
	tests := []struct {
		method, path, body string
		status             int
		contains           string
	}{
		{http.MethodPost, "/api/v1/orders", `{"quotationId":"` + orderID + `","customerId":"customer-001","deliveryAddress":"HCMC"}`, http.StatusCreated, `"status":"WAITING_DEPOSIT"`},
		{http.MethodGet, "/api/v1/orders/" + orderID, "", http.StatusOK, `"orderId":"` + orderID + `"`},
		{http.MethodGet, "/api/v1/orders/" + orderID + "/timeline", "", http.StatusOK, `"status":"WAITING_DEPOSIT"`},
		{http.MethodGet, "/internal/orders/" + orderID + "/payment-summary", "", http.StatusOK, `"depositAmountVnd":700`},
		{http.MethodGet, "/internal/orders/" + orderID + "/warehouse-summary", "", http.StatusOK, `"customerId":"customer-001"`},
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

func TestOrderHandlerErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"validation", domain.ErrInvalidInput, http.StatusBadRequest, "VALIDATION_ERROR"},
		{"not found", domain.ErrOrderNotFound, http.StatusNotFound, "NOT_FOUND"},
		{"duplicate", domain.ErrQuotationConflict, http.StatusConflict, "CONFLICT"},
		{"invalid state", domain.ErrInvalidQuotation, http.StatusConflict, "INVALID_STATE"},
		{"dependency", domain.ErrDependency, http.StatusBadGateway, "DEPENDENCY_ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler(fakeService{err: tt.err}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+orderID, nil))
			if response.Code != tt.status || !strings.Contains(response.Body.String(), tt.code) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestCreateRejectsMalformedBody(t *testing.T) {
	response := httptest.NewRecorder()
	handler(fakeService{}).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(`{`)))
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "VALIDATION_ERROR") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
