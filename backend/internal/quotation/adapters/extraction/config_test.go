package extraction

import (
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := LoadConfig(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != ModeDemo || cfg.FetchTimeout != 1200*time.Millisecond || cfg.MaxResponseBytes != 1_048_576 || cfg.MaxRedirects != 3 || len(cfg.AllowedHosts) != 0 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadConfigHybrid(t *testing.T) {
	values := map[string]string{
		"PRODUCT_EXTRACTOR_MODE":     "hybrid",
		"PRODUCT_ALLOWED_HOSTS":      "store.example,*.shop.example",
		"PRODUCT_FETCH_TIMEOUT":      "900ms",
		"PRODUCT_MAX_RESPONSE_BYTES": "2048",
		"PRODUCT_MAX_REDIRECTS":      "2",
	}
	cfg, err := LoadConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != ModeHybrid || len(cfg.AllowedHosts) != 2 || cfg.FetchTimeout != 900*time.Millisecond || cfg.MaxResponseBytes != 2048 || cfg.MaxRedirects != 2 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadConfigRejectsMalformedValues(t *testing.T) {
	tests := []map[string]string{
		{"PRODUCT_EXTRACTOR_MODE": "all"},
		{"PRODUCT_EXTRACTOR_MODE": "hybrid"},
		{"PRODUCT_ALLOWED_HOSTS": "https://store.example"},
		{"PRODUCT_FETCH_TIMEOUT": "forever"},
		{"PRODUCT_MAX_RESPONSE_BYTES": "0"},
		{"PRODUCT_MAX_REDIRECTS": "-1"},
	}
	for _, values := range tests {
		if _, err := LoadConfig(func(key string) string { return values[key] }); err == nil {
			t.Fatalf("expected error for %v", values)
		}
	}
}

func TestHostMatcherExactAndExplicitWildcard(t *testing.T) {
	matcher, err := NewHostMatcher([]string{"store.example", "*.shop.example"})
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"store.example", "item.shop.example", "deep.item.shop.example"} {
		if !matcher.Match(host) {
			t.Fatalf("expected %s to match", host)
		}
	}
	for _, host := range []string{"www.store.example", "shop.example", "evilshop.example"} {
		if matcher.Match(host) {
			t.Fatalf("did not expect %s to match", host)
		}
	}
}
