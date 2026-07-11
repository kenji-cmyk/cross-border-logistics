package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/example/cross-border-logistics/internal/quotation/domain"
)

type AdminHTTPExchangeRates struct {
	endpoint string
	client   *http.Client
}

func NewAdminHTTPExchangeRates(endpoint string, timeout time.Duration) *AdminHTTPExchangeRates {
	return &AdminHTTPExchangeRates{
		endpoint: strings.TrimSpace(endpoint),
		client:   &http.Client{Timeout: timeout},
	}
}

func (p *AdminHTTPExchangeRates) Rate(ctx context.Context, currency string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.endpoint, nil)
	if err != nil {
		return 0, domain.ErrExchangeUnavailable
	}
	req.Header.Set("Accept", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return 0, domain.ErrExchangeUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return 0, domain.ErrExchangeUnavailable
	}
	var envelope struct {
		Data struct {
			ExchangeRates map[string]int64 `json:"exchangeRates"`
		} `json:"data"`
	}
	if json.NewDecoder(resp.Body).Decode(&envelope) != nil {
		return 0, domain.ErrExchangeUnavailable
	}
	rate, ok := envelope.Data.ExchangeRates[strings.ToUpper(strings.TrimSpace(currency))]
	if !ok {
		return 0, domain.ErrUnsupportedCurrency
	}
	if rate <= 0 {
		return 0, domain.ErrExchangeUnavailable
	}
	return rate, nil
}
