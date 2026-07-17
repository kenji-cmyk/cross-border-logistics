package vietcombank

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kenji-cmyk/cross-border-logistics/internal/admin/domain"
)

const (
	DefaultEndpoint = "https://portal.vietcombank.com.vn/Usercontrols/TVPortal.TyGia/pXML.aspx"
	MinimumCacheTTL = 5 * time.Minute
	maxResponseSize = 1 << 20
)

type SnapshotProvider interface {
	GetSystemRates(context.Context) (domain.SystemRates, error)
}

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type Provider struct {
	base     SnapshotProvider
	endpoint string
	client   HTTPDoer
	cacheTTL time.Duration
	now      func() time.Time

	mu        sync.Mutex
	cached    domain.SystemRates
	expiresAt time.Time
	hasCache  bool
	lastErr   error
}

func New(base SnapshotProvider, endpoint string, client HTTPDoer, cacheTTL time.Duration) (*Provider, error) {
	if base == nil {
		return nil, fmt.Errorf("base rates provider is required")
	}
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return nil, fmt.Errorf("Vietcombank exchange-rate endpoint must be an absolute HTTPS URL")
	}
	if client == nil {
		return nil, fmt.Errorf("HTTP client is required")
	}
	if cacheTTL < MinimumCacheTTL {
		return nil, fmt.Errorf("Vietcombank exchange-rate cache TTL must be at least %s", MinimumCacheTTL)
	}
	return &Provider{base: base, endpoint: parsed.String(), client: client, cacheTTL: cacheTTL, now: time.Now}, nil
}

func (p *Provider) GetSystemRates(ctx context.Context) (domain.SystemRates, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := p.now()
	if p.hasCache && now.Before(p.expiresAt) {
		return clone(p.cached), nil
	}
	if !p.hasCache && p.lastErr != nil && now.Before(p.expiresAt) {
		return domain.SystemRates{}, p.lastErr
	}

	base, err := p.base.GetSystemRates(ctx)
	if err != nil {
		return domain.SystemRates{}, err
	}
	live, effectiveAt, err := p.fetch(ctx, base.SupportedCurrencies, now)
	if err != nil {
		p.expiresAt = now.Add(p.cacheTTL)
		if p.hasCache {
			return clone(p.cached), nil
		}
		p.lastErr = err
		return domain.SystemRates{}, p.lastErr
	}
	base.ExchangeRates = live
	base.EffectiveAt = effectiveAt
	if err := base.Validate(); err != nil {
		return domain.SystemRates{}, fmt.Errorf("invalid Vietcombank exchange-rate snapshot: %w", err)
	}

	p.cached = clone(base)
	p.expiresAt = now.Add(p.cacheTTL)
	p.hasCache = true
	p.lastErr = nil
	return clone(base), nil
}

type exchangeRateList struct {
	DateTime string         `xml:"DateTime"`
	Rates    []exchangeRate `xml:"Exrate"`
}

type exchangeRate struct {
	Currency string `xml:"CurrencyCode,attr"`
	Sell     string `xml:"Sell,attr"`
}

func (p *Provider) fetch(ctx context.Context, currencies []string, now time.Time) (map[string]int64, time.Time, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.endpoint, nil)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("create Vietcombank exchange-rate request: %w", err)
	}
	req.Header.Set("Accept", "application/xml,text/xml")
	req.Header.Set("User-Agent", "cross-border-logistics/1.0")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("fetch Vietcombank exchange rates: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, time.Time{}, fmt.Errorf("Vietcombank exchange-rate provider returned HTTP %d", resp.StatusCode)
	}

	var payload exchangeRateList
	reader := io.LimitReader(resp.Body, maxResponseSize+1)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("read Vietcombank exchange rates: %w", err)
	}
	if len(data) > maxResponseSize {
		return nil, time.Time{}, fmt.Errorf("Vietcombank exchange-rate response is too large")
	}
	if err := xml.Unmarshal(data, &payload); err != nil {
		return nil, time.Time{}, fmt.Errorf("decode Vietcombank exchange rates: %w", err)
	}

	available := make(map[string]int64, len(payload.Rates))
	for _, item := range payload.Rates {
		code := strings.ToUpper(strings.TrimSpace(item.Currency))
		rate, parseErr := parseSellRate(item.Sell)
		if code != "" && parseErr == nil {
			available[code] = rate
		}
	}
	selected := make(map[string]int64, len(currencies))
	for _, currency := range currencies {
		code := strings.ToUpper(strings.TrimSpace(currency))
		rate, ok := available[code]
		if !ok {
			return nil, time.Time{}, fmt.Errorf("Vietcombank did not return a selling rate for %s", code)
		}
		selected[code] = rate
	}

	effectiveAt := now.UTC()
	if parsed, parseErr := time.ParseInLocation("1/2/2006 3:04:05 PM", strings.TrimSpace(payload.DateTime), time.FixedZone("ICT", 7*60*60)); parseErr == nil {
		effectiveAt = parsed.UTC()
	}
	return selected, effectiveAt, nil
}

func parseSellRate(value string) (int64, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(value), ",", "")
	parsed, err := strconv.ParseFloat(normalized, 64)
	if err != nil || parsed <= 0 || math.IsInf(parsed, 0) || math.IsNaN(parsed) {
		return 0, fmt.Errorf("invalid selling rate %q", value)
	}
	return int64(math.Round(parsed)), nil
}

func clone(source domain.SystemRates) domain.SystemRates {
	result := source
	result.SupportedCurrencies = append([]string(nil), source.SupportedCurrencies...)
	result.ExchangeRates = make(map[string]int64, len(source.ExchangeRates))
	for currency, rate := range source.ExchangeRates {
		result.ExchangeRates[currency] = rate
	}
	return result
}
