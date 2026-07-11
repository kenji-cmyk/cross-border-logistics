package ports

import (
	"context"
	"time"

	"github.com/example/cross-border-logistics/internal/quotation/domain"
)

type QuotationRepository interface {
	Create(context.Context, domain.Quotation) error
	FindByID(context.Context, string) (domain.Quotation, error)
}

type ConfirmationRepository interface {
	Confirm(context.Context, string, string, time.Time) (domain.Quotation, error)
}

type ExchangeRateProvider interface {
	Rate(context.Context, string) (int64, error)
}

type ExtractedProduct struct {
	URL, Name, SourcePrice, Currency, ImageURL string
}

type ProductExtractor interface {
	Extract(context.Context, string) (ExtractedProduct, error)
}

type ProductRestrictionChecker interface {
	IsRestricted(context.Context, string, string) bool
}
