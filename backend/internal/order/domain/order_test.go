package domain_test

import (
	"testing"

	"github.com/kenji-cmyk/cross-border-logistics/internal/order/domain"
)

func TestCanTransition(t *testing.T) {
	tests := []struct {
		name     string
		from, to domain.OrderStatus
		want     bool
	}{
		{"deposit paid", domain.StatusWaitingDeposit, domain.StatusWaitingPurchase, true},
		{"purchase completed", domain.StatusWaitingPurchase, domain.StatusPurchased, true},
		{"phase six warehouse event", domain.StatusWaitingPurchase, domain.StatusArrivedForeignWarehouse, true},
		{"skip states", domain.StatusWaitingDeposit, domain.StatusArrivedForeignWarehouse, false},
		{"delivered is terminal", domain.StatusDelivered, domain.StatusWaitingPurchase, false},
		{"cancelled is terminal", domain.StatusCancelled, domain.StatusWaitingPurchase, false},
		{"active order can cancel", domain.StatusWaitingPurchase, domain.StatusCancelled, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := domain.CanTransition(tt.from, tt.to); got != tt.want {
				t.Fatalf("CanTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}
