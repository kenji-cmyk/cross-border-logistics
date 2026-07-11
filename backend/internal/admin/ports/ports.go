package ports

import (
	"context"

	"github.com/example/cross-border-logistics/internal/admin/domain"
)

type RatesProvider interface {
	GetSystemRates(context.Context) (domain.SystemRates, error)
}
