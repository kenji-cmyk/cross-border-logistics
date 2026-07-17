package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kenji-cmyk/cross-border-logistics/internal/quotation/adapters"
	"github.com/kenji-cmyk/cross-border-logistics/internal/quotation/application"
	"github.com/kenji-cmyk/cross-border-logistics/internal/quotation/domain"
	"github.com/kenji-cmyk/cross-border-logistics/internal/quotation/ports"
)

type fakeRepository struct {
	values  map[string]domain.Quotation
	creates int
}

func (f *fakeRepository) Create(_ context.Context, q domain.Quotation) error {
	f.creates++
	if f.values == nil {
		f.values = map[string]domain.Quotation{}
	}
	f.values[q.ID] = q
	return nil
}
func (f *fakeRepository) FindByID(_ context.Context, id string) (domain.Quotation, error) {
	q, ok := f.values[id]
	if !ok {
		return domain.Quotation{}, domain.ErrQuotationNotFound
	}
	return q, nil
}

type fakeExtractor struct {
	product ports.ExtractedProduct
}

func (f fakeExtractor) Extract(_ context.Context, productURL string) (ports.ExtractedProduct, error) {
	product := f.product
	if product.URL == "" {
		product.URL = productURL
	}
	return product, nil
}

func validInput() (application.ExtractInput, ports.ExtractedProduct) {
	return application.ExtractInput{CustomerID: "customer-001", ProductURL: "https://example.com/product/123", Quantity: 1}, ports.ExtractedProduct{Name: "Wireless Keyboard", SourcePrice: "50", Currency: "usd"}
}

func newService(repository *fakeRepository, product ...ports.ExtractedProduct) *application.Service {
	extracted := ports.ExtractedProduct{Name: "Wireless Keyboard", SourcePrice: "50", Currency: "usd"}
	if len(product) > 0 {
		extracted = product[0]
	}
	return application.NewService(repository, adapters.MockExchangeRates{}, adapters.MockRestrictionChecker{}, fakeExtractor{product: extracted})
}

func TestExtractQuotationCalculations(t *testing.T) {
	tests := []struct {
		name               string
		mutate             func(*application.ExtractInput, *ports.ExtractedProduct)
		amount, fee, total int64
	}{
		{name: "USD quotation", amount: 1_300_000, fee: 65_000, total: 1_485_000},
		{name: "quantity greater than one", mutate: func(input *application.ExtractInput, _ *ports.ExtractedProduct) { input.Quantity = 3 }, amount: 3_900_000, fee: 195_000, total: 4_215_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, product := validInput()
			if tt.mutate != nil {
				tt.mutate(&input, &product)
			}
			result, err := newService(&fakeRepository{}, product).Extract(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if result.ProductAmountVND != tt.amount || result.ServiceFeeVND != tt.fee || result.EstimatedShippingFeeVND != 120_000 || result.TotalAmountVND != tt.total {
				t.Fatalf("unexpected amounts: %+v", result)
			}
			if result.Status != domain.StatusPendingConfirmation {
				t.Fatalf("status = %s", result.Status)
			}
			if result.Currency != "USD" {
				t.Fatalf("currency = %s", result.Currency)
			}
		})
	}
}

func TestExtractQuotationRejectsInvalidInputWithoutSaving(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*application.ExtractInput, *ports.ExtractedProduct)
		target error
	}{
		{name: "unsupported currency", mutate: func(_ *application.ExtractInput, product *ports.ExtractedProduct) { product.Currency = "EUR" }, target: domain.ErrUnsupportedCurrency},
		{name: "missing customer", mutate: func(input *application.ExtractInput, _ *ports.ExtractedProduct) { input.CustomerID = "" }, target: domain.ErrInvalidQuotationInput},
		{name: "missing product URL", mutate: func(input *application.ExtractInput, _ *ports.ExtractedProduct) { input.ProductURL = "" }, target: domain.ErrInvalidQuotationInput},
		{name: "missing product name", mutate: func(_ *application.ExtractInput, product *ports.ExtractedProduct) { product.Name = "" }, target: domain.ErrInvalidQuotationInput},
		{name: "zero source price", mutate: func(_ *application.ExtractInput, product *ports.ExtractedProduct) { product.SourcePrice = "0" }, target: domain.ErrInvalidQuotationInput},
		{name: "zero quantity", mutate: func(input *application.ExtractInput, _ *ports.ExtractedProduct) { input.Quantity = 0 }, target: domain.ErrInvalidQuotationInput},
		{name: "restricted name", mutate: func(_ *application.ExtractInput, product *ports.ExtractedProduct) { product.Name = "toy gun" }, target: domain.ErrRestrictedProduct},
		{name: "restricted URL", mutate: func(_ *application.ExtractInput, product *ports.ExtractedProduct) {
			product.URL = "https://example.com/weapon"
		}, target: domain.ErrRestrictedProduct},
		{name: "case insensitive restriction", mutate: func(_ *application.ExtractInput, product *ports.ExtractedProduct) {
			product.Name = "Dangerous-Chemical kit"
		}, target: domain.ErrRestrictedProduct},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepository{}
			input, product := validInput()
			tt.mutate(&input, &product)
			_, err := newService(repo, product).Extract(context.Background(), input)
			if !errors.Is(err, tt.target) {
				t.Fatalf("error = %v, want %v", err, tt.target)
			}
			if repo.creates != 0 {
				t.Fatalf("repository creates = %d", repo.creates)
			}
		})
	}
}

func TestGetQuotationNotFound(t *testing.T) {
	_, err := newService(&fakeRepository{values: map[string]domain.Quotation{}}).Get(context.Background(), "46ab7a1a-bab7-4a46-b9f9-d7572a284895")
	if !errors.Is(err, domain.ErrQuotationNotFound) {
		t.Fatalf("error = %v", err)
	}
}
