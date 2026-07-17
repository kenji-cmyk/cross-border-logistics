package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/kenji-cmyk/cross-border-logistics/internal/order/application"
	"github.com/kenji-cmyk/cross-border-logistics/internal/order/domain"
	"github.com/kenji-cmyk/cross-border-logistics/pkg/httpx"
)

type Service interface {
	Create(context.Context, application.CreateInput) (domain.Order, error)
	Get(context.Context, string) (domain.Order, error)
	Timeline(context.Context, string) ([]domain.TrackingEvent, error)
	GetPaymentSummary(context.Context, string) (application.PaymentSummary, error)
	GetWarehouseSummary(context.Context, string) (application.WarehouseSummary, error)
}

type Handler struct {
	service Service
	logger  *slog.Logger
}

type createRequest struct {
	QuotationID     string `json:"quotationId"`
	CustomerID      string `json:"customerId"`
	DeliveryAddress string `json:"deliveryAddress"`
}

func New(service Service, logger *slog.Logger, serviceName string) http.Handler {
	h := &Handler{service: service, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, httpx.Health{Status: "UP", Service: serviceName})
	})
	mux.HandleFunc("POST /api/v1/orders", h.create)
	mux.HandleFunc("GET /api/v1/orders/{orderId}", h.get)
	mux.HandleFunc("GET /api/v1/orders/{orderId}/timeline", h.timeline)
	mux.HandleFunc("GET /internal/orders/{orderId}/payment-summary", h.paymentSummary)
	mux.HandleFunc("GET /internal/orders/{orderId}/warehouse-summary", h.warehouseSummary)
	return mux
}

func (h *Handler) warehouseSummary(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.GetWarehouseSummary(r.Context(), r.PathValue("orderId"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteSuccess(w, r, http.StatusOK, result)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var request createRequest
	if err := decoder.Decode(&request); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "request body must be valid JSON", nil)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "request body must contain one JSON object", nil)
		return
	}
	result, err := h.service.Create(r.Context(), application.CreateInput{QuotationID: request.QuotationID, CustomerID: request.CustomerID, DeliveryAddress: request.DeliveryAddress})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteSuccess(w, r, http.StatusCreated, result)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Get(r.Context(), r.PathValue("orderId"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteSuccess(w, r, http.StatusOK, result)
}

func (h *Handler) timeline(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Timeline(r.Context(), r.PathValue("orderId"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteSuccess(w, r, http.StatusOK, result)
}

func (h *Handler) paymentSummary(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.GetPaymentSummary(r.Context(), r.PathValue("orderId"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteSuccess(w, r, http.StatusOK, result)
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "order input is invalid", nil)
	case errors.Is(err, domain.ErrOrderNotFound), errors.Is(err, domain.ErrQuotationNotFound):
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "order or quotation was not found", nil)
	case errors.Is(err, domain.ErrQuotationConflict):
		httpx.WriteError(w, r, http.StatusConflict, "CONFLICT", "quotation already has an order", nil)
	case errors.Is(err, domain.ErrCustomerMismatch):
		httpx.WriteError(w, r, http.StatusConflict, "CONFLICT", "quotation does not belong to customer", nil)
	case errors.Is(err, domain.ErrInvalidQuotation), errors.Is(err, domain.ErrInvalidTransition):
		httpx.WriteError(w, r, http.StatusConflict, "INVALID_STATE", "quotation or order is not in a valid state", nil)
	case errors.Is(err, domain.ErrDependency):
		httpx.WriteError(w, r, http.StatusBadGateway, "DEPENDENCY_ERROR", "quotation service is unavailable", nil)
	default:
		h.logger.ErrorContext(r.Context(), "order request failed", "request_id", httpx.RequestID(r.Context()), "error", err)
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "an internal error occurred", nil)
	}
}
