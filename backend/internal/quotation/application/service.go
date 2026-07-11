package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/example/cross-border-logistics/internal/quotation/domain"
	"github.com/example/cross-border-logistics/internal/quotation/ports"
	"github.com/google/uuid"
)

type CreateInput struct {
	CustomerID, ProductURL, ProductName, SourcePrice, Currency string
	Quantity                                                   int
}

type ExtractInput struct {
	CustomerID, ProductURL string
	Quantity               int
}

type Result struct {
	ID                      string        `json:"id"`
	CustomerID              string        `json:"customerId"`
	ProductURL              string        `json:"productUrl"`
	ProductName             string        `json:"productName"`
	ImageURL                string        `json:"imageUrl,omitempty"`
	SourcePrice             Decimal       `json:"sourcePrice"`
	Currency                string        `json:"currency"`
	Quantity                int           `json:"quantity"`
	ExchangeRate            int64         `json:"exchangeRate"`
	ProductAmountVND        int64         `json:"productAmountVnd"`
	ServiceFeeVND           int64         `json:"serviceFeeVnd"`
	EstimatedShippingFeeVND int64         `json:"estimatedShippingFeeVnd"`
	TotalAmountVND          int64         `json:"totalAmountVnd"`
	Status                  domain.Status `json:"status"`
	CreatedAt               time.Time     `json:"createdAt"`
	UpdatedAt               time.Time     `json:"updatedAt"`
}

type Snapshot struct {
	QuotationID    string        `json:"quotationId"`
	CustomerID     string        `json:"customerId"`
	ProductURL     string        `json:"productUrl"`
	ProductName    string        `json:"productName"`
	Quantity       int           `json:"quantity"`
	TotalAmountVND int64         `json:"totalAmountVnd"`
	Status         domain.Status `json:"status"`
	CreatedAt      time.Time     `json:"createdAt"`
}

type Decimal string

func (d Decimal) MarshalJSON() ([]byte, error) { return []byte(d), nil }

type Service struct {
	repository   ports.QuotationRepository
	rates        ports.ExchangeRateProvider
	restrictions ports.ProductRestrictionChecker
	extractor    ports.ProductExtractor
	now          func() time.Time
}

func NewService(r ports.QuotationRepository, rates ports.ExchangeRateProvider, restrictions ports.ProductRestrictionChecker, extractors ...ports.ProductExtractor) *Service {
	var extractor ports.ProductExtractor
	if len(extractors) > 0 {
		extractor = extractors[0]
	}
	return &Service{repository: r, rates: rates, restrictions: restrictions, extractor: extractor, now: time.Now}
}

func (s *Service) Extract(ctx context.Context, input ExtractInput) (Result, error) {
	if s.extractor == nil {
		return Result{}, domain.ErrExtractionUnavailable
	}
	input.CustomerID, input.ProductURL = strings.TrimSpace(input.CustomerID), strings.TrimSpace(input.ProductURL)
	if input.CustomerID == "" || input.ProductURL == "" || input.Quantity <= 0 {
		return Result{}, domain.ErrInvalidQuotationInput
	}
	product, err := s.extractor.Extract(ctx, input.ProductURL)
	if err != nil {
		return Result{}, err
	}
	result, err := s.create(ctx, CreateInput{CustomerID: input.CustomerID, ProductURL: product.URL, ProductName: product.Name, SourcePrice: product.SourcePrice, Currency: product.Currency, Quantity: input.Quantity}, product.ImageURL)
	return result, err
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Result, error) {
	return s.create(ctx, input, "")
}

func (s *Service) create(ctx context.Context, input CreateInput, imageURL string) (Result, error) {
	input.CustomerID, input.ProductURL, input.ProductName = strings.TrimSpace(input.CustomerID), strings.TrimSpace(input.ProductURL), strings.TrimSpace(input.ProductName)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	if input.CustomerID == "" || input.ProductURL == "" || input.ProductName == "" || input.Currency == "" || input.Quantity <= 0 {
		return Result{}, domain.ErrInvalidQuotationInput
	}
	price, err := domain.ParseSourcePrice(input.SourcePrice)
	if err != nil {
		return Result{}, domain.ErrInvalidQuotationInput
	}
	if s.restrictions.IsRestricted(ctx, input.ProductName, input.ProductURL) {
		return Result{}, domain.ErrRestrictedProduct
	}
	rate, err := s.rates.Rate(ctx, input.Currency)
	if err != nil {
		return Result{}, err
	}
	amount, fee, total, err := domain.Calculate(price, input.Quantity, rate)
	if err != nil {
		return Result{}, err
	}
	now := s.now().UTC()
	q := domain.Quotation{ID: uuid.NewString(), CustomerID: input.CustomerID, ProductURL: input.ProductURL, ProductName: input.ProductName, ImageURL: imageURL, SourcePriceMicros: price, Currency: input.Currency, Quantity: input.Quantity, ExchangeRate: rate, ProductAmountVND: amount, ServiceFeeVND: fee, EstimatedShippingFeeVND: domain.EstimatedShippingFeeVND, TotalAmountVND: total, Status: domain.StatusPendingConfirmation, CreatedAt: now, UpdatedAt: now}
	if err := s.repository.Create(ctx, q); err != nil {
		return Result{}, err
	}
	return toResult(q), nil
}

func (s *Service) Get(ctx context.Context, id string) (Result, error) {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return Result{}, domain.ErrInvalidQuotationInput
	}
	q, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return Result{}, err
	}
	return toResult(q), nil
}

func (s *Service) GetSnapshot(ctx context.Context, id string) (Snapshot, error) {
	r, err := s.Get(ctx, id)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{QuotationID: r.ID, CustomerID: r.CustomerID, ProductURL: r.ProductURL, ProductName: r.ProductName, Quantity: r.Quantity, TotalAmountVND: r.TotalAmountVND, Status: r.Status, CreatedAt: r.CreatedAt}, nil
}

func (s *Service) Confirm(ctx context.Context, quotationID, orderID string) (Snapshot, error) {
	if _, err := uuid.Parse(strings.TrimSpace(quotationID)); err != nil {
		return Snapshot{}, domain.ErrInvalidQuotationInput
	}
	if _, err := uuid.Parse(strings.TrimSpace(orderID)); err != nil {
		return Snapshot{}, domain.ErrInvalidQuotationInput
	}
	q, err := s.repository.FindByID(ctx, quotationID)
	if err != nil {
		return Snapshot{}, err
	}
	if s.now().UTC().Sub(q.CreatedAt) > 30*time.Minute {
		return Snapshot{}, domain.ErrInvalidQuotationInput
	}
	confirmations, ok := s.repository.(ports.ConfirmationRepository)
	if !ok {
		return Snapshot{}, domain.ErrQuotationConflict
	}
	q, err = confirmations.Confirm(ctx, quotationID, orderID, s.now().UTC())
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{QuotationID: q.ID, CustomerID: q.CustomerID, ProductURL: q.ProductURL, ProductName: q.ProductName, Quantity: q.Quantity, TotalAmountVND: q.TotalAmountVND, Status: q.Status, CreatedAt: q.CreatedAt}, nil
}

func toResult(q domain.Quotation) Result {
	return Result{ID: q.ID, CustomerID: q.CustomerID, ProductURL: q.ProductURL, ProductName: q.ProductName, ImageURL: q.ImageURL, SourcePrice: Decimal(domain.FormatSourcePrice(q.SourcePriceMicros)), Currency: q.Currency, Quantity: q.Quantity, ExchangeRate: q.ExchangeRate, ProductAmountVND: q.ProductAmountVND, ServiceFeeVND: q.ServiceFeeVND, EstimatedShippingFeeVND: q.EstimatedShippingFeeVND, TotalAmountVND: q.TotalAmountVND, Status: q.Status, CreatedAt: q.CreatedAt, UpdatedAt: q.UpdatedAt}
}

func IsNotFound(err error) bool { return errors.Is(err, domain.ErrQuotationNotFound) }
