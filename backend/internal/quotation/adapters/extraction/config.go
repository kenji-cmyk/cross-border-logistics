package extraction

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	ModeDemo   = "demo"
	ModeHybrid = "hybrid"
)

type Config struct {
	Mode             string
	AllowedHosts     []string
	FetchTimeout     time.Duration
	MaxResponseBytes int64
	MaxRedirects     int
}

func LoadConfig(getenv func(string) string) (Config, error) {
	cfg := Config{
		Mode:             valueOrDefault(getenv("PRODUCT_EXTRACTOR_MODE"), ModeDemo),
		FetchTimeout:     1200 * time.Millisecond,
		MaxResponseBytes: 1_048_576,
		MaxRedirects:     3,
	}
	cfg.Mode = strings.ToLower(strings.TrimSpace(cfg.Mode))
	if cfg.Mode != ModeDemo && cfg.Mode != ModeHybrid {
		return Config{}, fmt.Errorf("PRODUCT_EXTRACTOR_MODE must be demo or hybrid")
	}

	var err error
	if raw := strings.TrimSpace(getenv("PRODUCT_FETCH_TIMEOUT")); raw != "" {
		cfg.FetchTimeout, err = time.ParseDuration(raw)
		if err != nil || cfg.FetchTimeout <= 0 {
			return Config{}, fmt.Errorf("PRODUCT_FETCH_TIMEOUT must be a positive duration")
		}
	}
	if raw := strings.TrimSpace(getenv("PRODUCT_MAX_RESPONSE_BYTES")); raw != "" {
		cfg.MaxResponseBytes, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || cfg.MaxResponseBytes <= 0 {
			return Config{}, fmt.Errorf("PRODUCT_MAX_RESPONSE_BYTES must be a positive integer")
		}
	}
	if raw := strings.TrimSpace(getenv("PRODUCT_MAX_REDIRECTS")); raw != "" {
		cfg.MaxRedirects, err = strconv.Atoi(raw)
		if err != nil || cfg.MaxRedirects < 0 {
			return Config{}, fmt.Errorf("PRODUCT_MAX_REDIRECTS must be a non-negative integer")
		}
	}

	matcher, err := NewHostMatcher(splitHosts(getenv("PRODUCT_ALLOWED_HOSTS")))
	if err != nil {
		return Config{}, fmt.Errorf("PRODUCT_ALLOWED_HOSTS: %w", err)
	}
	cfg.AllowedHosts = matcher.Patterns()
	if cfg.Mode == ModeHybrid && len(cfg.AllowedHosts) == 0 {
		return Config{}, fmt.Errorf("PRODUCT_ALLOWED_HOSTS must contain at least one host in hybrid mode")
	}
	return cfg, nil
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func splitHosts(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return strings.Split(raw, ",")
}
