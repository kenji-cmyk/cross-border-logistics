package http_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	warehousehttp "github.com/kenji-cmyk/cross-border-logistics/internal/warehouse/adapters/http"
	"github.com/kenji-cmyk/cross-border-logistics/internal/warehouse/application"
	"github.com/kenji-cmyk/cross-border-logistics/internal/warehouse/domain"
	"github.com/kenji-cmyk/cross-border-logistics/pkg/httpx"
	"github.com/google/uuid"
)

type fakeService struct{ err error }

func (f fakeService) ReceivePackage(context.Context, application.ReceivePackageInput) (domain.Package, error) {
	return samplePackage(), f.err
}
func (f fakeService) Get(context.Context, string) (domain.Package, error) {
	return samplePackage(), f.err
}
func samplePackage() domain.Package {
	return domain.Package{ID: uuid.MustParse("46ab7a1a-bab7-4a46-b9f9-d7572a284895"), OrderID: uuid.New(), Status: domain.StatusReceivedAtForeignWarehouse}
}
func handler(service fakeService) http.Handler {
	return httpx.RequestIDMiddleware(warehousehttp.New(service, slog.New(slog.NewTextHandler(io.Discard, nil)), "warehouse-service"))
}

func TestGetPackage(t *testing.T) {
	response := httptest.NewRecorder()
	handler(fakeService{}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/warehouse/packages/46ab7a1a-bab7-4a46-b9f9-d7572a284895", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"RECEIVED_AT_FOREIGN_WAREHOUSE"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestGetPackageNotFound(t *testing.T) {
	response := httptest.NewRecorder()
	handler(fakeService{err: domain.ErrPackageNotFound}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/warehouse/packages/46ab7a1a-bab7-4a46-b9f9-d7572a284895", nil))
	if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), "NOT_FOUND") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
