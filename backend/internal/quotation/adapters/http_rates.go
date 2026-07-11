package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/example/cross-border-logistics/internal/quotation/domain"
)

type HTTPExchangeRates struct {
	baseURL  string
	client   *http.Client
	attempts int
}

func NewHTTPExchangeRates(baseURL string, timeout time.Duration) *HTTPExchangeRates {
	return &HTTPExchangeRates{baseURL: strings.TrimRight(baseURL, "/"), client: &http.Client{Timeout: timeout}, attempts: 3}
}

func (p *HTTPExchangeRates) Rate(ctx context.Context, currency string) (int64, error) {
	var last error
	for attempt := 0; attempt < p.attempts; attempt++ {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/rates/"+url.PathEscape(strings.ToUpper(currency)), nil)
		resp, err := p.client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				var body struct {
					Rate int64 `json:"rate"`
				}
				if json.NewDecoder(resp.Body).Decode(&body) == nil && body.Rate > 0 {
					return body.Rate, nil
				}
				return 0, domain.ErrExchangeUnavailable
			}
			if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
				return 0, domain.ErrUnsupportedCurrency
			}
			last = fmt.Errorf("rate provider HTTP %d", resp.StatusCode)
		} else {
			last = err
		}
		if attempt < p.attempts-1 {
			select {
			case <-ctx.Done():
				return 0, domain.ErrExchangeUnavailable
			case <-time.After(time.Duration(50*(1<<attempt)+rand.IntN(25)) * time.Millisecond):
			}
		}
	}
	_ = last
	return 0, domain.ErrExchangeUnavailable
}
