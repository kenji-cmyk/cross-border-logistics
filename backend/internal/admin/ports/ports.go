package ports

import (
	"context"

	"github.com/kenji-cmyk/cross-border-logistics/internal/admin/domain"
)

type RatesProvider interface {
	GetSystemRates(context.Context) (domain.SystemRates, error)
}
