package quotationclient_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/example/cross-border-logistics/internal/order/adapters/quotationclient"
	"github.com/example/cross-border-logistics/internal/order/domain"
)

func TestGetQuotation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/quotations/q-1" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"quotationId":"q-1","customerId":"c-1","productUrl":"https://example.com/p","productName":"Keyboard","quantity":2,"totalAmountVnd":1485000,"status":"PENDING_CONFIRMATION","createdAt":"2026-07-11T04:30:00Z"},"meta":{"requestId":"r-1"}}`))
	}))
	defer server.Close()
	snapshot, err := quotationclient.New(server.URL, time.Second).GetQuotation(context.Background(), "q-1")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.QuotationID != "q-1" || snapshot.Quantity != 2 || snapshot.TotalAmountVND != 1_485_000 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
}

func TestGetQuotationMapsErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"not found", http.StatusNotFound, `{}`, domain.ErrQuotationNotFound},
		{"server error", http.StatusInternalServerError, `{}`, domain.ErrDependency},
		{"invalid response", http.StatusOK, `{`, domain.ErrDependency},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			_, err := quotationclient.New(server.URL, time.Second).GetQuotation(context.Background(), "q-1")
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}
