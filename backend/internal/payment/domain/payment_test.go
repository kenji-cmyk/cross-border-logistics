package domain_test

import (
	"testing"

	"github.com/example/cross-border-logistics/internal/payment/domain"
)

func TestCanTransition(t *testing.T) {
	tests := []struct {
		from, to domain.PaymentStatus
		want     bool
	}{
		{domain.StatusPending, domain.StatusSucceeded, true},
		{domain.StatusPending, domain.StatusFailed, false},
		{domain.StatusPending, domain.StatusCancelled, false},
		{domain.StatusSucceeded, domain.StatusRefunded, false},
		{domain.StatusSucceeded, domain.StatusPending, false},
		{domain.StatusSucceeded, domain.StatusSucceeded, false},
	}
	for _, tt := range tests {
		if got := domain.CanTransition(tt.from, tt.to); got != tt.want {
			t.Fatalf("CanTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}
