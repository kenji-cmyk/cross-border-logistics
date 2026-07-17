package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/kenji-cmyk/cross-border-logistics/internal/warehouse/application"
	"github.com/kenji-cmyk/cross-border-logistics/internal/warehouse/domain"
	"github.com/kenji-cmyk/cross-border-logistics/pkg/httpx"
)

type Service interface {
	ReceivePackage(context.Context, application.ReceivePackageInput) (domain.Package, error)
	Get(context.Context, string) (domain.Package, error)
}
type Handler struct {
	service Service
	logger  *slog.Logger
}
type receiveRequest struct {
	OrderID              string  `json:"orderId"`
	SourceTrackingNumber string  `json:"sourceTrackingNumber"`
	WarehouseCode        string  `json:"warehouseCode"`
	WeightKg             float64 `json:"weightKg"`
	LengthCm             float64 `json:"lengthCm"`
	WidthCm              float64 `json:"widthCm"`
	HeightCm             float64 `json:"heightCm"`
}

func New(service Service, logger *slog.Logger, serviceName string) http.Handler {
	h := &Handler{service: service, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, httpx.Health{Status: "UP", Service: serviceName})
	})
	mux.HandleFunc("POST /api/v1/warehouse/packages/receive", h.receive)
	mux.HandleFunc("GET /api/v1/warehouse/packages/{packageId}", h.get)
	return mux
}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	pkg, err := h.service.Get(r.Context(), r.PathValue("packageId"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteSuccess(w, r, http.StatusOK, pkg)
}
func (h *Handler) receive(w http.ResponseWriter, r *http.Request) {
	d := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	d.DisallowUnknownFields()
	var q receiveRequest
	if err := d.Decode(&q); err != nil {
		httpx.WriteError(w, r, 400, "VALIDATION_ERROR", "request body must be valid JSON", nil)
		return
	}
	var extra any
	if err := d.Decode(&extra); !errors.Is(err, io.EOF) {
		httpx.WriteError(w, r, 400, "VALIDATION_ERROR", "request body must contain one JSON object", nil)
		return
	}
	p, err := h.service.ReceivePackage(r.Context(), application.ReceivePackageInput{OrderID: q.OrderID, SourceTrackingNumber: q.SourceTrackingNumber, WarehouseCode: q.WarehouseCode, WeightKg: q.WeightKg, LengthCm: q.LengthCm, WidthCm: q.WidthCm, HeightCm: q.HeightCm, RequestID: httpx.RequestID(r.Context())})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteSuccess(w, r, http.StatusCreated, p)
}
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidPackageInput), errors.Is(err, domain.ErrInvalidTrackingNumber), errors.Is(err, domain.ErrInvalidWarehouseCode), errors.Is(err, domain.ErrInvalidPackageDimensions):
		httpx.WriteError(w, r, 400, "VALIDATION_ERROR", "package input is invalid", nil)
	case errors.Is(err, domain.ErrOrderNotFound), errors.Is(err, domain.ErrPackageNotFound):
		httpx.WriteError(w, r, 404, "NOT_FOUND", "order or package was not found", nil)
	case errors.Is(err, domain.ErrPackageAlreadyExists):
		httpx.WriteError(w, r, 409, "PACKAGE_ALREADY_EXISTS", "source tracking number already has a package", nil)
	case errors.Is(err, domain.ErrOrderNotEligibleForWarehouse):
		httpx.WriteError(w, r, 409, "INVALID_STATE", "order cannot receive a package", nil)
	case errors.Is(err, domain.ErrOrderServiceUnavailable):
		httpx.WriteError(w, r, 502, "DEPENDENCY_ERROR", "order service is unavailable", nil)
	default:
		h.logger.ErrorContext(r.Context(), "warehouse request failed", "request_id", httpx.RequestID(r.Context()), "error", err)
		httpx.WriteError(w, r, 500, "INTERNAL_ERROR", "an internal error occurred", nil)
	}
}
