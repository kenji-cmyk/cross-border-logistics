package config

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/example/cross-border-logistics/internal/admin/domain"
)

type LookupFunc func(string) (string, bool)

type Provider struct{ rates domain.SystemRates }

func Load(lookup LookupFunc, effectiveAt time.Time) (*Provider, error) {
	serviceFee, err := integer(lookup, "ADMIN_SERVICE_FEE_PERCENT", 5)
	if err != nil {
		return nil, err
	}
	shippingFee, err := integer64(lookup, "ADMIN_ESTIMATED_SHIPPING_FEE_VND", 120000)
	if err != nil {
		return nil, err
	}
	deposit, err := integer(lookup, "ADMIN_DEPOSIT_PERCENT", 70)
	if err != nil {
		return nil, err
	}

	currencies := split(raw(lookup, "ADMIN_SUPPORTED_CURRENCIES", "USD,CNY,JPY,KRW"))
	rates := make(map[string]int64, len(currencies))
	for _, currency := range currencies {
		name := "ADMIN_EXCHANGE_RATE_" + currency
		fallback, hasDefault := defaultExchangeRates[currency]
		value, exists := lookup(name)
		if !exists && !hasDefault {
			continue
		}
		if !exists {
			value = strconv.FormatInt(fallback, 10)
		}
		parsed, parseErr := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if parseErr != nil {
			return nil, fmt.Errorf("parse %s: %w", name, parseErr)
		}
		rates[currency] = parsed
	}

	snapshot := domain.SystemRates{
		ServiceFeePercent:       serviceFee,
		EstimatedShippingFeeVND: shippingFee,
		DepositPercent:          deposit,
		SupportedCurrencies:     currencies,
		ExchangeRates:           rates,
		EffectiveAt:             effectiveAt.UTC(),
	}
	if err := snapshot.Validate(); err != nil {
		return nil, fmt.Errorf("invalid admin rates configuration: %w", err)
	}
	return &Provider{rates: snapshot}, nil
}

var defaultExchangeRates = map[string]int64{"USD": 26000, "CNY": 3600, "JPY": 175, "KRW": 19}

func (p *Provider) GetSystemRates(context.Context) (domain.SystemRates, error) {
	result := p.rates
	result.SupportedCurrencies = append([]string(nil), p.rates.SupportedCurrencies...)
	result.ExchangeRates = make(map[string]int64, len(p.rates.ExchangeRates))
	for currency, rate := range p.rates.ExchangeRates {
		result.ExchangeRates[currency] = rate
	}
	return result, nil
}

func integer(lookup LookupFunc, name string, fallback int) (int, error) {
	value := raw(lookup, name, strconv.Itoa(fallback))
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return parsed, nil
}

func integer64(lookup LookupFunc, name string, fallback int64) (int64, error) {
	value := raw(lookup, name, strconv.FormatInt(fallback, 10))
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return parsed, nil
}

func raw(lookup LookupFunc, name, fallback string) string {
	if value, exists := lookup(name); exists {
		return strings.TrimSpace(value)
	}
	return fallback
}

func split(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		currency := strings.ToUpper(strings.TrimSpace(part))
		if currency == "" {
			continue
		}
		if _, exists := seen[currency]; exists {
			continue
		}
		seen[currency] = struct{}{}
		result = append(result, currency)
	}
	return result
}
