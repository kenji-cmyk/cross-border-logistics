package http

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/kenji-cmyk/cross-border-logistics/internal/admin/application"
	"github.com/kenji-cmyk/cross-border-logistics/pkg/httpx"
)

type RatesUseCase interface {
	Execute(context.Context) (application.SystemRates, error)
}

type Handler struct {
	rates  RatesUseCase
	logger *slog.Logger
}

func New(rates RatesUseCase, logger *slog.Logger, serviceName string) http.Handler {
	h := &Handler{rates: rates, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, httpx.Health{Status: "UP", Service: serviceName})
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, httpx.Health{Status: "UP", Service: serviceName})
	})
	mux.HandleFunc("GET /api/v1/admin/rates", h.getRates)
	mux.HandleFunc("GET /api/v1/admin/rates/", h.getRates)
	return mux
}

func (h *Handler) getRates(w http.ResponseWriter, r *http.Request) {
	rates, err := h.rates.Execute(r.Context())
	if err != nil {
		h.logger.ErrorContext(r.Context(), "admin rates request failed", "request_id", httpx.RequestID(r.Context()), "error", err)
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "unable to load system rates", nil)
		return
	}
	httpx.WriteSuccess(w, r, http.StatusOK, rates)
}
