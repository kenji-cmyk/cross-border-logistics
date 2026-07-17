package orderclient_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kenji-cmyk/cross-border-logistics/internal/payment/adapters/orderclient"
	"github.com/kenji-cmyk/cross-border-logistics/internal/payment/domain"
)

func TestGetPaymentSummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/orders/o-1/payment-summary" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":{"orderId":"o-1","depositAmountVnd":700,"remainingAmountVnd":300,"status":"WAITING_DEPOSIT"},"meta":{"requestId":"r-1"}}`))
	}))
	defer server.Close()
	summary, err := orderclient.New(server.URL, time.Second).GetPaymentSummary(context.Background(), "o-1")
	if err != nil {
		t.Fatal(err)
	}
	if summary.OrderID != "o-1" || summary.DepositAmountVND != 700 || summary.RemainingAmountVND != 300 || summary.Status != "WAITING_DEPOSIT" {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestGetPaymentSummaryMapsErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"not found", http.StatusNotFound, `{}`, domain.ErrOrderNotFound},
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
			_, err := orderclient.New(server.URL, time.Second).GetPaymentSummary(context.Background(), "o-1")
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}
