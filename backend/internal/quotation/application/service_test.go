package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/example/cross-border-logistics/internal/quotation/adapters"
	"github.com/example/cross-border-logistics/internal/quotation/application"
	"github.com/example/cross-border-logistics/internal/quotation/domain"
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

func validInput() application.CreateInput {
	return application.CreateInput{CustomerID: "customer-001", ProductURL: "https://example.com/product/123", ProductName: "Wireless Keyboard", SourcePrice: "50", Currency: "usd", Quantity: 1}
}
func newService(repository *fakeRepository) *application.Service {
	return application.NewService(repository, adapters.MockExchangeRates{}, adapters.MockRestrictionChecker{})
}

func TestCreateQuotationCalculations(t *testing.T) {
	tests := []struct {
		name               string
		mutate             func(*application.CreateInput)
		amount, fee, total int64
	}{
		{name: "USD quotation", amount: 1_300_000, fee: 65_000, total: 1_485_000},
		{name: "quantity greater than one", mutate: func(i *application.CreateInput) { i.Quantity = 3 }, amount: 3_900_000, fee: 195_000, total: 4_215_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validInput()
			if tt.mutate != nil {
				tt.mutate(&input)
			}
			result, err := newService(&fakeRepository{}).Create(context.Background(), input)
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

func TestCreateQuotationRejectsInvalidInputWithoutSaving(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*application.CreateInput)
		target error
	}{
		{name: "unsupported currency", mutate: func(i *application.CreateInput) { i.Currency = "EUR" }, target: domain.ErrUnsupportedCurrency},
		{name: "missing customer", mutate: func(i *application.CreateInput) { i.CustomerID = "" }, target: domain.ErrInvalidQuotationInput},
		{name: "missing product URL", mutate: func(i *application.CreateInput) { i.ProductURL = "" }, target: domain.ErrInvalidQuotationInput},
		{name: "missing product name", mutate: func(i *application.CreateInput) { i.ProductName = "" }, target: domain.ErrInvalidQuotationInput},
		{name: "zero source price", mutate: func(i *application.CreateInput) { i.SourcePrice = "0" }, target: domain.ErrInvalidQuotationInput},
		{name: "zero quantity", mutate: func(i *application.CreateInput) { i.Quantity = 0 }, target: domain.ErrInvalidQuotationInput},
		{name: "restricted name", mutate: func(i *application.CreateInput) { i.ProductName = "toy gun" }, target: domain.ErrRestrictedProduct},
		{name: "restricted URL", mutate: func(i *application.CreateInput) { i.ProductURL = "https://example.com/weapon" }, target: domain.ErrRestrictedProduct},
		{name: "case insensitive restriction", mutate: func(i *application.CreateInput) { i.ProductName = "Dangerous-Chemical kit" }, target: domain.ErrRestrictedProduct},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepository{}
			input := validInput()
			tt.mutate(&input)
			_, err := newService(repo).Create(context.Background(), input)
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
