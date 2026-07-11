package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/example/cross-border-logistics/internal/quotation/application"
	"github.com/example/cross-border-logistics/internal/quotation/domain"
	"github.com/example/cross-border-logistics/pkg/httpx"
)

type Service interface {
	Create(context.Context, application.CreateInput) (application.Result, error)
	Extract(context.Context, application.ExtractInput) (application.Result, error)
	Get(context.Context, string) (application.Result, error)
	GetSnapshot(context.Context, string) (application.Snapshot, error)
	Confirm(context.Context, string, string) (application.Snapshot, error)
}

type Handler struct {
	service Service
	logger  *slog.Logger
}

func New(service Service, logger *slog.Logger, serviceName string) http.Handler {
	h := &Handler{service: service, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, httpx.Health{Status: "UP", Service: serviceName})
	})
	mux.HandleFunc("POST /api/v1/quotations/extract", h.extract)
	mux.HandleFunc("GET /api/v1/quotations/{quotationId}", h.get)
	mux.HandleFunc("GET /internal/quotations/{quotationId}", h.snapshot)
	mux.HandleFunc("POST /internal/quotations/{quotationId}/confirm", h.confirm)
	return mux
}

func (h *Handler) confirm(w http.ResponseWriter, r *http.Request) {
	var request struct {
		OrderID string `json:"orderId"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || ensureEOF(decoder) != nil {
		httpx.WriteError(w, r, 400, "VALIDATION_ERROR", "request body must be one valid JSON object", nil)
		return
	}
	result, err := h.service.Confirm(r.Context(), r.PathValue("quotationId"), request.OrderID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteSuccess(w, r, http.StatusOK, result)
}

type extractRequest struct {
	CustomerID string `json:"customerId"`
	ProductURL string `json:"productUrl"`
	Quantity   int    `json:"quantity"`
}

func (h *Handler) extract(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var request extractRequest
	if decoder.Decode(&request) != nil || ensureEOF(decoder) != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "request body must be one valid JSON object", nil)
		return
	}
	result, err := h.service.Extract(r.Context(), application.ExtractInput{CustomerID: request.CustomerID, ProductURL: request.ProductURL, Quantity: request.Quantity})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteSuccess(w, r, http.StatusOK, result)
}

type createRequest struct {
	CustomerID  string      `json:"customerId"`
	ProductURL  string      `json:"productUrl"`
	ProductName string      `json:"productName"`
	SourcePrice json.Number `json:"sourcePrice"`
	Currency    string      `json:"currency"`
	Quantity    int         `json:"quantity"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	var request createRequest
	if err := decoder.Decode(&request); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "request body must be valid JSON", nil)
		return
	}
	if err := ensureEOF(decoder); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "request body must contain one JSON object", nil)
		return
	}
	result, err := h.service.Create(r.Context(), application.CreateInput{CustomerID: request.CustomerID, ProductURL: request.ProductURL, ProductName: request.ProductName, SourcePrice: request.SourcePrice.String(), Currency: request.Currency, Quantity: request.Quantity})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteSuccess(w, r, http.StatusCreated, result)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Get(r.Context(), r.PathValue("quotationId"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteSuccess(w, r, http.StatusOK, result)
}
func (h *Handler) snapshot(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.GetSnapshot(r.Context(), r.PathValue("quotationId"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteSuccess(w, r, http.StatusOK, result)
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrRestrictedProduct):
		httpx.WriteError(w, r, http.StatusBadRequest, "RESTRICTED_ITEM", "product is restricted", nil)
	case errors.Is(err, domain.ErrUnsafeProductURL):
		httpx.WriteError(w, r, http.StatusBadRequest, "UNSAFE_PRODUCT_URL", "product URL is not allowed", nil)
	case errors.Is(err, domain.ErrExtractionUnavailable):
		httpx.WriteError(w, r, http.StatusBadGateway, "PRODUCT_EXTRACTION_UNAVAILABLE", "product could not be extracted", nil)
	case errors.Is(err, domain.ErrExchangeUnavailable):
		httpx.WriteError(w, r, http.StatusBadGateway, "EXCHANGE_RATE_UNAVAILABLE", "exchange rate provider is unavailable", nil)
	case errors.Is(err, domain.ErrQuotationConflict):
		httpx.WriteError(w, r, http.StatusConflict, "QUOTATION_ALREADY_CONFIRMED", "quotation is reserved by another order", nil)
	case errors.Is(err, domain.ErrUnsupportedCurrency):
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "currency is not supported", nil)
	case errors.Is(err, domain.ErrInvalidQuotationInput):
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "quotation input is invalid", nil)
	case errors.Is(err, domain.ErrQuotationNotFound):
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "quotation was not found", nil)
	default:
		h.logger.ErrorContext(r.Context(), "quotation request failed", "request_id", httpx.RequestID(r.Context()), "error", err)
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "an internal error occurred", nil)
	}
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("extra JSON value")
	}
	return err
}
