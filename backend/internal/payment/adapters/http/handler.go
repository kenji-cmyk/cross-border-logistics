package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/example/cross-border-logistics/internal/payment/application"
	"github.com/example/cross-border-logistics/internal/payment/domain"
	"github.com/example/cross-border-logistics/pkg/httpx"
)

type Service interface {
	CreateDeposit(context.Context, application.CreateDepositInput) (domain.Payment, error)
	Get(context.Context, string) (domain.Payment, error)
	MarkSucceeded(context.Context, string) (domain.Payment, error)
}

type CallbackService interface {
	ProcessCallback(context.Context, string, string, string) (domain.Payment, error)
}

type Handler struct {
	service       Service
	logger        *slog.Logger
	webhookSecret string
	demo          bool
}

type createDepositRequest struct {
	OrderID string `json:"orderId"`
}

func New(service Service, logger *slog.Logger, serviceName string, options ...string) http.Handler {
	secret, env := "demo-webhook-secret", "development"
	if len(options) > 0 && strings.TrimSpace(options[0]) != "" {
		secret = options[0]
	}
	if len(options) > 1 {
		env = options[1]
	}
	h := &Handler{service: service, logger: logger, webhookSecret: secret, demo: env == "development" || env == "demo"}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, httpx.Health{Status: "UP", Service: serviceName})
	})
	mux.HandleFunc("POST /api/v1/payments/deposit", h.createDeposit)
	mux.HandleFunc("POST /api/v1/payments/callback", h.callback)
	mux.HandleFunc("GET /api/v1/payments/{paymentId}", h.get)
	if h.demo {
		mux.HandleFunc("POST /api/v1/payments/{paymentId}/mock-success", h.mockSuccess)
	}
	return mux
}

func (h *Handler) callback(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		httpx.WriteError(w, r, 400, "VALIDATION_ERROR", "invalid callback body", nil)
		return
	}
	parts := strings.Split(r.Header.Get("X-Webhook-Signature"), ",")
	values := map[string]string{}
	for _, part := range parts {
		pair := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(pair) == 2 {
			values[pair[0]] = pair[1]
		}
	}
	timestamp, err := strconv.ParseInt(values["t"], 10, 64)
	if err != nil || time.Since(time.Unix(timestamp, 0)) > 5*time.Minute || time.Until(time.Unix(timestamp, 0)) > time.Minute {
		httpx.WriteError(w, r, 401, "INVALID_WEBHOOK_SIGNATURE", "webhook signature is invalid or stale", nil)
		return
	}
	mac := hmac.New(sha256.New, []byte(h.webhookSecret))
	mac.Write([]byte(values["t"] + "."))
	mac.Write(body)
	expected := mac.Sum(nil)
	supplied, err := hex.DecodeString(values["v1"])
	if err != nil || !hmac.Equal(expected, supplied) {
		httpx.WriteError(w, r, 401, "INVALID_WEBHOOK_SIGNATURE", "webhook signature is invalid or stale", nil)
		return
	}
	var request struct {
		EventID           string `json:"eventId"`
		ProviderReference string `json:"providerReference"`
		Status            string `json:"status"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil {
		httpx.WriteError(w, r, 400, "VALIDATION_ERROR", "invalid callback body", nil)
		return
	}
	callbacks, ok := h.service.(CallbackService)
	if !ok {
		httpx.WriteError(w, r, 503, "CALLBACK_UNAVAILABLE", "callback processing is unavailable", nil)
		return
	}
	result, err := callbacks.ProcessCallback(r.Context(), request.ProviderReference, request.EventID, request.Status)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteSuccess(w, r, 200, result)
}

func (h *Handler) createDeposit(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.service.CreateDeposit(r.Context(), application.CreateDepositInput{OrderID: request.OrderID})
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
		httpx.WriteError(w, r, http.StatusConflict, "CONFLICT", "deposit payment already exists", nil)
	case errors.Is(err, domain.ErrInvalidState):
		httpx.WriteError(w, r, http.StatusConflict, "INVALID_STATE", "order or payment is not in a valid state", nil)
	case errors.Is(err, domain.ErrDependency):
		httpx.WriteError(w, r, http.StatusBadGateway, "DEPENDENCY_ERROR", "order service is unavailable", nil)
	default:
		h.logger.ErrorContext(r.Context(), "payment request failed", "request_id", httpx.RequestID(r.Context()), "error", err)
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "an internal error occurred", nil)
	}
}
