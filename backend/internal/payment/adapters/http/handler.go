package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/example/cross-border-logistics/internal/payment/application"
	"github.com/example/cross-border-logistics/internal/payment/domain"
	"github.com/example/cross-border-logistics/pkg/httpx"
)

type Service interface {
	CreateDeposit(context.Context, application.CreateDepositInput) (domain.Payment, error)
	Get(context.Context, string) (domain.Payment, error)
	MarkSucceeded(context.Context, string) (domain.Payment, error)
}

type Handler struct {
	service Service
	logger  *slog.Logger
}

type createDepositRequest struct {
	OrderID string `json:"orderId"`
}

func New(service Service, logger *slog.Logger, serviceName string) http.Handler {
	h := &Handler{service: service, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, httpx.Health{Status: "UP", Service: serviceName})
	})
	mux.HandleFunc("POST /api/v1/payments/deposits", h.createDeposit)
	mux.HandleFunc("GET /api/v1/payments/{paymentId}", h.get)
	mux.HandleFunc("POST /api/v1/payments/{paymentId}/mock-success", h.mockSuccess)
	return mux
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
