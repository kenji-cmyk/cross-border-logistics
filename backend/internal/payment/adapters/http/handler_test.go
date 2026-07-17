package http_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	paymenthttp "github.com/kenji-cmyk/cross-border-logistics/internal/payment/adapters/http"
	"github.com/kenji-cmyk/cross-border-logistics/internal/payment/application"
	"github.com/kenji-cmyk/cross-border-logistics/internal/payment/domain"
	"github.com/kenji-cmyk/cross-border-logistics/internal/payment/ports"
	"github.com/kenji-cmyk/cross-border-logistics/pkg/httpx"
)

const paymentID = "9f42fc31-e997-4b6f-a742-981ca145bacc"

type fakeService struct{ err error }

func (f fakeService) CreateDeposit(context.Context, application.CreateDepositInput) (domain.Payment, error) {
	return domain.Payment{ID: paymentID, OrderID: "46ab7a1a-bab7-4a46-b9f9-d7572a284895", Type: domain.TypeDeposit, AmountVND: 700, Currency: domain.CurrencyVND, Status: domain.StatusPending}, f.err
}
func (f fakeService) CreateRemainingBalance(context.Context, application.CreateRemainingBalanceInput) (domain.Payment, error) {
	return domain.Payment{ID: paymentID, OrderID: "46ab7a1a-bab7-4a46-b9f9-d7572a284895", Type: domain.TypeRemainingBalance, AmountVND: 300, Currency: domain.CurrencyVND, Status: domain.StatusPending}, f.err
}
func (f fakeService) Get(context.Context, string) (domain.Payment, error) {
	return domain.Payment{ID: paymentID, Status: domain.StatusPending}, f.err
}
func (f fakeService) MarkSucceeded(context.Context, string) (domain.Payment, error) {
	return domain.Payment{ID: paymentID, Status: domain.StatusSucceeded}, f.err
}
func (f fakeService) ProcessSePayWebhook(context.Context, application.SePayWebhookInput) (domain.Payment, error) {
	return domain.Payment{ID: paymentID, Status: domain.StatusSucceeded}, f.err
}

type pgService struct {
	fakeService
	input application.SePayWebhookInput
}

func (s *pgService) BuildCheckout(context.Context, string) (ports.CheckoutForm, error) {
	return ports.CheckoutForm{
		Action: "https://pay-sandbox.sepay.vn/v1/checkout/init",
		Fields: []ports.CheckoutField{{Name: "merchant", Value: "SP-TEST-123"}, {Name: "signature", Value: "signed-value"}},
	}, s.err
}

func (s *pgService) ProcessSePayWebhook(_ context.Context, input application.SePayWebhookInput) (domain.Payment, error) {
	s.input = input
	return domain.Payment{ID: paymentID, Status: domain.StatusSucceeded}, s.err
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
		{http.MethodPost, "/api/v1/payments/deposit", `{"orderId":"46ab7a1a-bab7-4a46-b9f9-d7572a284895"}`, http.StatusCreated, `"status":"PENDING"`},
		{http.MethodPost, "/api/v1/payments/remaining-balance", `{"orderId":"46ab7a1a-bab7-4a46-b9f9-d7572a284895"}`, http.StatusCreated, `"type":"REMAINING_BALANCE"`},
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

func TestSePayWebhookHMACAndResponseContract(t *testing.T) {
	secret, account := "sepay-secret", "0123456789"
	body := `{"id":92704,"gateway":"MBBank","transactionDate":"2026-07-15 09:30:00","accountNumber":"0123456789","subAccount":"","code":"CBL9F42FC31E997","content":"CBL9F42FC31E997","transferType":"in","description":"","transferAmount":700,"accumulated":1000,"referenceCode":"FT123"}`
	timestamp := time.Now().Unix()
	timestampHeader := strconv.FormatInt(timestamp, 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestampHeader + "." + body))
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := httpx.RequestIDMiddleware(paymenthttp.New(fakeService{}, logger, "payment-service", secret, "development", account, "sepay"))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/payments/sepay/webhook", strings.NewReader(body))
	request.Header.Set("X-SePay-Timestamp", timestampHeader)
	request.Header.Set("X-SePay-Signature", signature)
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) != `{"success":true}` {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}

	mockResponse := httptest.NewRecorder()
	h.ServeHTTP(mockResponse, httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+paymentID+"/mock-success", strings.NewReader(`{}`)))
	if mockResponse.Code != http.StatusNotFound {
		t.Fatalf("mock success must be disabled for SePay; status=%d", mockResponse.Code)
	}

	badBody := strings.Replace(body, account, "9999999999", 1)
	badMac := hmac.New(sha256.New, []byte(secret))
	badMac.Write([]byte(timestampHeader + "." + badBody))
	badRequest := httptest.NewRequest(http.MethodPost, "/api/v1/payments/sepay/webhook", strings.NewReader(badBody))
	badRequest.Header.Set("X-SePay-Timestamp", timestampHeader)
	badRequest.Header.Set("X-SePay-Signature", "sha256="+hex.EncodeToString(badMac.Sum(nil)))
	badResponse := httptest.NewRecorder()
	h.ServeHTTP(badResponse, badRequest)
	if badResponse.Code != http.StatusBadRequest {
		t.Fatalf("wrong destination account status=%d body=%s", badResponse.Code, badResponse.Body.String())
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
	handler(fakeService{}).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/payments/deposit", strings.NewReader(`{`)))
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "VALIDATION_ERROR") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestSePayPGCheckoutRendersAutoSubmitForm(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := &pgService{}
	h := paymenthttp.New(service, logger, "payment-service", "", "development", "", "sepay_pg", "pg-secret")
	response := httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/payments/"+paymentID+"/checkout", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, expected := range []string{`action="https://pay-sandbox.sepay.vn/v1/checkout/init"`, `name="merchant" value="SP-TEST-123"`, `name="signature" value="signed-value"`, `sepay-checkout`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("checkout page is missing %q: %s", expected, body)
		}
	}
	if !strings.Contains(response.Header().Get("Content-Security-Policy"), "form-action https://pay-sandbox.sepay.vn") {
		t.Fatalf("unexpected CSP: %s", response.Header().Get("Content-Security-Policy"))
	}
}

func TestSePayPGIPNAuthenticatesAndProcessesPaidOrder(t *testing.T) {
	const secret = "sandbox-secret"
	body := `{"timestamp":1757058220,"notification_type":"ORDER_PAID","order":{"id":"order-at-sepay","order_status":"CAPTURED","order_currency":"VND","order_amount":"700.00","order_invoice_number":"CBL9F42FC31E997"},"transaction":{"id":"384c66dd-41e6-4316-a544-b4141682595c","transaction_status":"APPROVED","transaction_amount":"700","transaction_currency":"VND"}}`
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := &pgService{}
	h := paymenthttp.New(service, logger, "payment-service", "", "development", "", "sepay_pg", secret)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/payments/sepay/pg/ipn", strings.NewReader(body))
	request.Header.Set("X-Secret-Key", secret)
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) != `{"success":true}` {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if service.input.ProviderReference != "CBL9F42FC31E997" || service.input.EventID != "sepay-pg:384c66dd-41e6-4316-a544-b4141682595c" || service.input.TransferAmountVND != 700 || service.input.TransferType != "in" {
		t.Fatalf("unexpected application input: %+v", service.input)
	}

	badSecret := httptest.NewRequest(http.MethodPost, "/api/v1/payments/sepay/pg/ipn", strings.NewReader(body))
	badSecret.Header.Set("X-Secret-Key", "wrong")
	badResponse := httptest.NewRecorder()
	h.ServeHTTP(badResponse, badSecret)
	if badResponse.Code != http.StatusUnauthorized {
		t.Fatalf("wrong secret status=%d body=%s", badResponse.Code, badResponse.Body.String())
	}

	badAmountBody := strings.Replace(body, `"transaction_amount":"700"`, `"transaction_amount":"699"`, 1)
	badAmount := httptest.NewRequest(http.MethodPost, "/api/v1/payments/sepay/pg/ipn", strings.NewReader(badAmountBody))
	badAmount.Header.Set("X-Secret-Key", secret)
	badAmountResponse := httptest.NewRecorder()
	h.ServeHTTP(badAmountResponse, badAmount)
	if badAmountResponse.Code != http.StatusBadRequest {
		t.Fatalf("amount mismatch status=%d body=%s", badAmountResponse.Code, badAmountResponse.Body.String())
	}
}
