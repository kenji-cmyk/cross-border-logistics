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
	Get(context.Context, string) (application.Result, error)
	GetSnapshot(context.Context, string) (application.Snapshot, error)
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
	mux.HandleFunc("POST /api/v1/quotations", h.create)
	mux.HandleFunc("GET /api/v1/quotations/{quotationId}", h.get)
	mux.HandleFunc("GET /internal/quotations/{quotationId}", h.snapshot)
	return mux
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
		httpx.WriteError(w, r, http.StatusBadRequest, "RESTRICTED_PRODUCT", "product is restricted", nil)
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
