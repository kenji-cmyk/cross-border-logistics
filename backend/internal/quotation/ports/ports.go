package ports

import (
	"context"

	"github.com/example/cross-border-logistics/internal/quotation/domain"
)

type QuotationRepository interface {
	Create(context.Context, domain.Quotation) error
	FindByID(context.Context, string) (domain.Quotation, error)
}

type ExchangeRateProvider interface {
	Rate(context.Context, string) (int64, error)
}

type ProductRestrictionChecker interface {
	IsRestricted(context.Context, string, string) bool
}
