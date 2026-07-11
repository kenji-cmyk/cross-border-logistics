package vietcombank

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/example/cross-border-logistics/internal/admin/domain"
)

type fakeBaseProvider struct{}

func (fakeBaseProvider) GetSystemRates(context.Context) (domain.SystemRates, error) {
	return domain.SystemRates{
		ServiceFeePercent:       5,
		EstimatedShippingFeeVND: 120000,
		DepositPercent:          70,
		SupportedCurrencies:     []string{"USD", "CNY", "JPY", "KRW"},
		ExchangeRates:           map[string]int64{"USD": 1, "CNY": 1, "JPY": 1, "KRW": 1},
	}, nil
}

type fakeDoer struct {
	responses []string
	err       error
	calls     int
}

func (f *fakeDoer) Do(request *http.Request) (*http.Response, error) {
	f.calls++
	if request.URL.Scheme != "https" || request.Header.Get("User-Agent") == "" {
		return nil, errors.New("unsafe request")
	}
	if f.err != nil {
		return nil, f.err
	}
	body := f.responses[0]
	if len(f.responses) > 1 {
		f.responses = f.responses[1:]
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
}

const validXML = `<ExrateList>
<DateTime>7/12/2026 4:04:31 AM</DateTime>
<Exrate CurrencyCode="USD" Sell="26,470.00" />
<Exrate CurrencyCode="CNY" Sell="3,939.58" />
<Exrate CurrencyCode="JPY" Sell="167.79" />
<Exrate CurrencyCode="KRW" Sell="18.21" />
</ExrateList>`

func TestProviderUsesSellingRatesRoundsToVNDAndCaches(t *testing.T) {
	doer := &fakeDoer{responses: []string{validXML}}
	provider, err := New(fakeBaseProvider{}, DefaultEndpoint, doer, MinimumCacheTTL)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 12, 5, 0, 0, 0, time.FixedZone("ICT", 7*60*60))
	provider.now = func() time.Time { return now }

	first, err := provider.GetSystemRates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := provider.GetSystemRates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if doer.calls != 1 {
		t.Fatalf("HTTP calls = %d", doer.calls)
	}
	for currency, expected := range map[string]int64{"USD": 26470, "CNY": 3940, "JPY": 168, "KRW": 18} {
		if first.ExchangeRates[currency] != expected || second.ExchangeRates[currency] != expected {
			t.Fatalf("rate %s: first=%d second=%d", currency, first.ExchangeRates[currency], second.ExchangeRates[currency])
		}
	}
	expectedTime := time.Date(2026, 7, 11, 21, 4, 31, 0, time.UTC)
	if !first.EffectiveAt.Equal(expectedTime) {
		t.Fatalf("effectiveAt = %s", first.EffectiveAt)
	}
}

func TestProviderReturnsStaleSnapshotWhenRefreshFails(t *testing.T) {
	doer := &fakeDoer{responses: []string{validXML}}
	provider, _ := New(fakeBaseProvider{}, DefaultEndpoint, doer, MinimumCacheTTL)
	now := time.Now()
	provider.now = func() time.Time { return now }
	first, err := provider.GetSystemRates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(MinimumCacheTTL + time.Second)
	doer.err = errors.New("provider unavailable")
	stale, err := provider.GetSystemRates(context.Background())
	if err != nil || stale.ExchangeRates["USD"] != first.ExchangeRates["USD"] {
		t.Fatalf("stale=%+v err=%v", stale, err)
	}
}

func TestProviderRejectsInvalidConfigurationAndIncompleteResponse(t *testing.T) {
	if _, err := New(fakeBaseProvider{}, "http://example.com/rates", &fakeDoer{}, MinimumCacheTTL); err == nil {
		t.Fatal("expected HTTPS validation error")
	}
	if _, err := New(fakeBaseProvider{}, DefaultEndpoint, &fakeDoer{}, time.Minute); err == nil {
		t.Fatal("expected cache TTL validation error")
	}
	doer := &fakeDoer{responses: []string{`<ExrateList><Exrate CurrencyCode="USD" Sell="26,470.00" /></ExrateList>`}}
	provider, _ := New(fakeBaseProvider{}, DefaultEndpoint, doer, MinimumCacheTTL)
	if _, err := provider.GetSystemRates(context.Background()); err == nil || !strings.Contains(err.Error(), "CNY") {
		t.Fatalf("error = %v", err)
	}
	if _, err := provider.GetSystemRates(context.Background()); err == nil || doer.calls != 1 {
		t.Fatalf("negative cache error=%v calls=%d", err, doer.calls)
	}
}

func TestParseSellRate(t *testing.T) {
	for input, expected := range map[string]int64{"26,470.00": 26470, "167.79": 168, "18.21": 18} {
		actual, err := parseSellRate(input)
		if err != nil || actual != expected {
			t.Fatalf("parseSellRate(%q) = %d, %v", input, actual, err)
		}
	}
	for _, input := range []string{"-", "0", "not-a-rate"} {
		if _, err := parseSellRate(input); err == nil {
			t.Fatalf("expected error for %q", input)
		}
	}
}
