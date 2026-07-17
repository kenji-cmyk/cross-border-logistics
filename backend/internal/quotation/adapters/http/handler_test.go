package http_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/example/cross-border-logistics/internal/quotation/adapters"
	quotationhttp "github.com/example/cross-border-logistics/internal/quotation/adapters/http"
	"github.com/example/cross-border-logistics/internal/quotation/application"
	"github.com/example/cross-border-logistics/internal/quotation/domain"
	"github.com/example/cross-border-logistics/pkg/httpx"
)

type repository struct{ values map[string]domain.Quotation }

func (r *repository) Create(_ context.Context, q domain.Quotation) error {
	r.values[q.ID] = q
	return nil
}
func (r *repository) FindByID(_ context.Context, id string) (domain.Quotation, error) {
	q, ok := r.values[id]
	if !ok {
		return domain.Quotation{}, domain.ErrQuotationNotFound
	}
	return q, nil
}
func setup() (http.Handler, *repository) {
	repo := &repository{values: map[string]domain.Quotation{}}
	service := application.NewService(repo, adapters.MockExchangeRates{}, adapters.MockRestrictionChecker{}, adapters.DemoProductExtractor{})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return httpx.RequestIDMiddleware(quotationhttp.New(service, logger, "quotation-service")), repo
}

func TestExtractAndGetQuotation(t *testing.T) {
	handler, _ := setup()
	body := `{"customerId":"customer-001","productUrl":"https://shop.example/p/1?name=Keyboard&price=50&currency=USD","quantity":1}`
	post := httptest.NewRecorder()
	handler.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/api/v1/quotations/extract", strings.NewReader(body)))
	if post.Code != http.StatusOK {
		t.Fatalf("POST status=%d body=%s", post.Code, post.Body.String())
	}
	if !strings.Contains(post.Body.String(), `"totalAmountVnd":1485000`) {
		t.Fatalf("demo quotation total changed: %s", post.Body.String())
	}
	var id string
	marker := `"id":"`
	start := strings.Index(post.Body.String(), marker)
	if start < 0 {
		t.Fatal("response has no id")
	}
	rest := post.Body.String()[start+len(marker):]
	id = rest[:strings.Index(rest, `"`)]
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/v1/quotations/"+id, nil))
	if get.Code != http.StatusOK {
		t.Fatalf("GET status=%d body=%s", get.Code, get.Body.String())
	}
	snapshot := httptest.NewRecorder()
	handler.ServeHTTP(snapshot, httptest.NewRequest(http.MethodGet, "/internal/quotations/"+id, nil))
	if snapshot.Code != http.StatusOK || !strings.Contains(snapshot.Body.String(), `"quotationId":"`+id+`"`) {
		t.Fatalf("snapshot status=%d body=%s", snapshot.Code, snapshot.Body.String())
	}
}

func TestExtractQuotationErrors(t *testing.T) {
	tests := []struct{ name, body, code string }{{"invalid JSON", `{`, "VALIDATION_ERROR"}, {"restricted", `{"customerId":"c","productUrl":"https://shop.example/gun?name=weapon&price=50&currency=USD","quantity":1}`, "RESTRICTED_ITEM"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, _ := setup()
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/quotations/extract", strings.NewReader(tt.body)))
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), tt.code) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestLegacyCreateQuotationRouteIsUnavailable(t *testing.T) {
	handler, _ := setup()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/quotations", strings.NewReader(`{}`)))
	if response.Code != http.StatusNotFound && response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestGetMissingQuotation(t *testing.T) {
	handler, _ := setup()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/quotations/46ab7a1a-bab7-4a46-b9f9-d7572a284895", nil))
	if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), "NOT_FOUND") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
