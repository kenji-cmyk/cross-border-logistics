package http

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kenji-cmyk/cross-border-logistics/internal/payment/application"
	"github.com/kenji-cmyk/cross-border-logistics/internal/payment/domain"
	"github.com/kenji-cmyk/cross-border-logistics/internal/payment/ports"
	"github.com/kenji-cmyk/cross-border-logistics/pkg/httpx"
)

type Service interface {
	CreateDeposit(context.Context, application.CreateDepositInput) (domain.Payment, error)
	CreateRemainingBalance(context.Context, application.CreateRemainingBalanceInput) (domain.Payment, error)
	Get(context.Context, string) (domain.Payment, error)
	MarkSucceeded(context.Context, string) (domain.Payment, error)
}

type SePayWebhookService interface {
	ProcessSePayWebhook(context.Context, application.SePayWebhookInput) (domain.Payment, error)
}

type CheckoutService interface {
	BuildCheckout(context.Context, string) (ports.CheckoutForm, error)
}

type Handler struct {
	service       Service
	logger        *slog.Logger
	webhookSecret string
	accountNumber string
	pgIPNSecret   string
	provider      string
	demo          bool
}

type createDepositRequest struct {
	OrderID string `json:"orderId"`
}

func New(service Service, logger *slog.Logger, serviceName string, options ...string) http.Handler {
	secret, env, accountNumber, provider, pgIPNSecret := "demo-webhook-secret", "development", "", "mock", ""
	if len(options) > 0 && strings.TrimSpace(options[0]) != "" {
		secret = options[0]
	}
	if len(options) > 1 {
		env = options[1]
	}
	if len(options) > 2 {
		accountNumber = strings.TrimSpace(options[2])
	}
	demo := env == "development" || env == "demo"
	if len(options) > 3 {
		provider = strings.ToLower(strings.TrimSpace(options[3]))
		demo = demo && provider == "mock"
	}
	if len(options) > 4 {
		pgIPNSecret = strings.TrimSpace(options[4])
	}
	h := &Handler{service: service, logger: logger, webhookSecret: secret, accountNumber: accountNumber, pgIPNSecret: pgIPNSecret, provider: provider, demo: demo}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, httpx.Health{Status: "UP", Service: serviceName})
	})
	mux.HandleFunc("POST /api/v1/payments/deposit", h.createDeposit)
	mux.HandleFunc("POST /api/v1/payments/remaining-balance", h.createRemainingBalance)
	if provider == "sepay" {
		mux.HandleFunc("POST /api/v1/payments/sepay/webhook", h.sePayWebhook)
		mux.HandleFunc("POST /api/v1/payments/callback", h.sePayWebhook)
	}
	if provider == "sepay_pg" {
		mux.HandleFunc("GET /api/v1/payments/{paymentId}/checkout", h.checkout)
		mux.HandleFunc("POST /api/v1/payments/sepay/pg/ipn", h.sePayPGIPN)
	}
	mux.HandleFunc("GET /api/v1/payments/{paymentId}", h.get)
	if h.demo {
		mux.HandleFunc("POST /api/v1/payments/{paymentId}/mock-success", h.mockSuccess)
	}
	return mux
}

var checkoutPage = template.Must(template.New("checkout").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>Continue to SePay</title>
  <style>body{font-family:system-ui,sans-serif;display:grid;min-height:100vh;place-items:center;margin:0;background:#f6f7fb;color:#111827}main{max-width:34rem;padding:2rem;text-align:center;background:#fff;border-radius:1.5rem;box-shadow:0 1rem 3rem #1118271a}button{border:0;border-radius:.9rem;background:#111827;color:#fff;padding:.9rem 1.25rem;font-weight:700;cursor:pointer}</style>
</head>
<body onload="document.getElementById('sepay-checkout').submit()">
  <main>
    <h1>Opening SePay secure checkout…</h1>
    <p>If you are not redirected automatically, continue below.</p>
    <form id="sepay-checkout" action="{{.Action}}" method="post">
      {{range .Fields}}<input type="hidden" name="{{.Name}}" value="{{.Value}}">{{end}}
      <button type="submit">Continue to SePay</button>
    </form>
  </main>
</body>
</html>`))

func (h *Handler) checkout(w http.ResponseWriter, r *http.Request) {
	service, ok := h.service.(CheckoutService)
	if !ok {
		h.writeError(w, r, domain.ErrCheckoutUnavailable)
		return
	}
	form, err := service.BuildCheckout(r.Context(), r.PathValue("paymentId"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	var page bytes.Buffer
	if err := checkoutPage.Execute(&page, form); err != nil {
		h.logger.ErrorContext(r.Context(), "render SePay checkout failed", "request_id", httpx.RequestID(r.Context()), "error", err)
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "an internal error occurred", nil)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; form-action https://pay-sandbox.sepay.vn https://pay.sepay.vn")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(page.Bytes())
}

func (h *Handler) sePayWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		httpx.WriteError(w, r, 400, "VALIDATION_ERROR", "invalid callback body", nil)
		return
	}
	timestampValue := strings.TrimSpace(r.Header.Get("X-SePay-Timestamp"))
	timestamp, err := strconv.ParseInt(timestampValue, 10, 64)
	if err != nil || time.Since(time.Unix(timestamp, 0)) > 5*time.Minute || time.Until(time.Unix(timestamp, 0)) > 5*time.Minute {
		httpx.WriteError(w, r, 401, "INVALID_WEBHOOK_SIGNATURE", "webhook signature is invalid or stale", nil)
		return
	}
	mac := hmac.New(sha256.New, []byte(h.webhookSecret))
	mac.Write([]byte(timestampValue + "."))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	supplied := strings.TrimSpace(r.Header.Get("X-SePay-Signature"))
	if !hmac.Equal([]byte(expected), []byte(supplied)) {
		httpx.WriteError(w, r, 401, "INVALID_WEBHOOK_SIGNATURE", "webhook signature is invalid or stale", nil)
		return
	}
	var request struct {
		ID             int64   `json:"id"`
		AccountNumber  string  `json:"accountNumber"`
		Code           *string `json:"code"`
		TransferType   string  `json:"transferType"`
		TransferAmount int64   `json:"transferAmount"`
	}
	if json.Unmarshal(body, &request) != nil || request.ID <= 0 || request.Code == nil || strings.TrimSpace(*request.Code) == "" || (h.accountNumber != "" && !hmac.Equal([]byte(h.accountNumber), []byte(strings.TrimSpace(request.AccountNumber)))) {
		httpx.WriteError(w, r, 400, "VALIDATION_ERROR", "invalid callback body", nil)
		return
	}
	callbacks, ok := h.service.(SePayWebhookService)
	if !ok {
		httpx.WriteError(w, r, 503, "CALLBACK_UNAVAILABLE", "callback processing is unavailable", nil)
		return
	}
	_, err = callbacks.ProcessSePayWebhook(r.Context(), application.SePayWebhookInput{
		ProviderReference: *request.Code,
		EventID:           fmt.Sprintf("sepay:%d", request.ID),
		TransferType:      request.TransferType,
		TransferAmountVND: request.TransferAmount,
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
}

type sePayPGIPNRequest struct {
	NotificationType string `json:"notification_type"`
	Order            struct {
		ID            string          `json:"id"`
		Status        string          `json:"order_status"`
		Currency      string          `json:"order_currency"`
		Amount        json.RawMessage `json:"order_amount"`
		InvoiceNumber string          `json:"order_invoice_number"`
	} `json:"order"`
	Transaction struct {
		ID       string          `json:"id"`
		Status   string          `json:"transaction_status"`
		Amount   json.RawMessage `json:"transaction_amount"`
		Currency string          `json:"transaction_currency"`
	} `json:"transaction"`
}

func (h *Handler) sePayPGIPN(w http.ResponseWriter, r *http.Request) {
	providedSecret := strings.TrimSpace(r.Header.Get("X-Secret-Key"))
	if h.pgIPNSecret == "" || !hmac.Equal([]byte(h.pgIPNSecret), []byte(providedSecret)) {
		httpx.WriteError(w, r, http.StatusUnauthorized, "INVALID_IPN_SECRET", "IPN authentication failed", nil)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid IPN body", nil)
		return
	}
	var request sePayPGIPNRequest
	if json.Unmarshal(body, &request) != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid IPN body", nil)
		return
	}
	if request.NotificationType != "ORDER_PAID" {
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}
	amount, err := parseSePayPGAmount(request.Order.Amount)
	if err != nil || request.Order.InvoiceNumber == "" || request.Order.Currency != "VND" || request.Order.Status != "CAPTURED" || request.Transaction.ID == "" || request.Transaction.Status != "APPROVED" || request.Transaction.Currency != "VND" {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid paid order IPN", nil)
		return
	}
	transactionAmount, err := parseSePayPGAmount(request.Transaction.Amount)
	if err != nil || transactionAmount != amount {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "IPN transaction amount does not match order", nil)
		return
	}
	callbacks, ok := h.service.(SePayWebhookService)
	if !ok {
		httpx.WriteError(w, r, http.StatusServiceUnavailable, "CALLBACK_UNAVAILABLE", "callback processing is unavailable", nil)
		return
	}
	_, err = callbacks.ProcessSePayWebhook(r.Context(), application.SePayWebhookInput{
		ProviderReference: request.Order.InvoiceNumber,
		EventID:           "sepay-pg:" + request.Transaction.ID,
		TransferType:      "in",
		TransferAmountVND: amount,
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func parseSePayPGAmount(raw json.RawMessage) (int64, error) {
	value := strings.TrimSpace(string(raw))
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		if err := json.Unmarshal(raw, &value); err != nil {
			return 0, err
		}
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && strings.Trim(parts[1], "0") != "") {
		return 0, fmt.Errorf("invalid VND amount")
	}
	amount, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || amount <= 0 {
		return 0, fmt.Errorf("invalid VND amount")
	}
	return amount, nil
}

func (h *Handler) createDeposit(w http.ResponseWriter, r *http.Request) {
	h.createPayment(w, r, domain.TypeDeposit)
}

func (h *Handler) createRemainingBalance(w http.ResponseWriter, r *http.Request) {
	h.createPayment(w, r, domain.TypeRemainingBalance)
}

func (h *Handler) createPayment(w http.ResponseWriter, r *http.Request, paymentType domain.PaymentType) {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var request createDepositRequest
	if err := decoder.Decode(&request); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "request body must be valid JSON", nil)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "request body must contain one JSON object", nil)
		return
	}
	var result domain.Payment
	var err error
	if paymentType == domain.TypeRemainingBalance {
		result, err = h.service.CreateRemainingBalance(r.Context(), application.CreateRemainingBalanceInput{OrderID: request.OrderID})
	} else {
		result, err = h.service.CreateDeposit(r.Context(), application.CreateDepositInput{OrderID: request.OrderID})
	}
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteSuccess(w, r, http.StatusCreated, result)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Get(r.Context(), r.PathValue("paymentId"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteSuccess(w, r, http.StatusOK, result)
}

func (h *Handler) mockSuccess(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.MarkSucceeded(r.Context(), r.PathValue("paymentId"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteSuccess(w, r, http.StatusOK, result)
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "payment input is invalid", nil)
	case errors.Is(err, domain.ErrPaymentNotFound), errors.Is(err, domain.ErrOrderNotFound):
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "payment or order was not found", nil)
	case errors.Is(err, domain.ErrPaymentConflict):
		httpx.WriteError(w, r, http.StatusConflict, "CONFLICT", "payment already exists for this order", nil)
	case errors.Is(err, domain.ErrInvalidState):
		httpx.WriteError(w, r, http.StatusConflict, "INVALID_STATE", "order or payment is not in a valid state", nil)
	case errors.Is(err, domain.ErrDependency):
		httpx.WriteError(w, r, http.StatusBadGateway, "DEPENDENCY_ERROR", "order service is unavailable", nil)
	case errors.Is(err, domain.ErrCheckoutUnavailable):
		httpx.WriteError(w, r, http.StatusConflict, "CHECKOUT_UNAVAILABLE", "hosted checkout is unavailable for this payment", nil)
	default:
		h.logger.ErrorContext(r.Context(), "payment request failed", "request_id", httpx.RequestID(r.Context()), "error", err)
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "an internal error occurred", nil)
	}
}
