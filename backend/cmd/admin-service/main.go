package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	adminconfig "github.com/kenji-cmyk/cross-border-logistics/internal/admin/adapters/config"
	adminhttp "github.com/kenji-cmyk/cross-border-logistics/internal/admin/adapters/http"
	"github.com/kenji-cmyk/cross-border-logistics/internal/admin/adapters/vietcombank"
	"github.com/kenji-cmyk/cross-border-logistics/internal/admin/application"
	"github.com/kenji-cmyk/cross-border-logistics/internal/admin/ports"
	"github.com/kenji-cmyk/cross-border-logistics/pkg/config"
	"github.com/kenji-cmyk/cross-border-logistics/pkg/httpx"
	"github.com/kenji-cmyk/cross-border-logistics/pkg/logger"
)

func main() {
	cfg, err := config.Load("admin-service", "ADMIN_SERVICE_PORT", "ADMIN_DB")
	if err != nil {
		log.Fatal(err)
	}

	serviceLogger := logger.New(cfg.ServiceName, cfg.AppEnv)
	fixedRates, err := adminconfig.Load(os.LookupEnv, time.Now())
	if err != nil {
		serviceLogger.Error("admin rates configuration failed", "error", err)
		log.Fatal(err)
	}
	var ratesProvider ports.RatesProvider = fixedRates
	providerMode := strings.ToLower(strings.TrimSpace(envOrDefault("EXCHANGE_RATE_PROVIDER", "fixed")))
	if providerMode == "vietcombank" {
		timeout, parseErr := time.ParseDuration(envOrDefault("EXCHANGE_RATE_FETCH_TIMEOUT", "2s"))
		if parseErr != nil || timeout <= 0 {
			log.Fatal("EXCHANGE_RATE_FETCH_TIMEOUT must be a positive duration")
		}
		cacheTTL, parseErr := time.ParseDuration(envOrDefault("EXCHANGE_RATE_CACHE_TTL", "5m"))
		if parseErr != nil {
			log.Fatal("EXCHANGE_RATE_CACHE_TTL must be a valid duration")
		}
		ratesProvider, err = vietcombank.New(
			fixedRates,
			envOrDefault("VIETCOMBANK_EXCHANGE_RATE_URL", vietcombank.DefaultEndpoint),
			&http.Client{Timeout: timeout},
			cacheTTL,
		)
		if err != nil {
			serviceLogger.Error("live exchange-rate configuration failed", "error", err)
			log.Fatal(err)
		}
	} else if providerMode != "fixed" {
		log.Fatalf("unsupported EXCHANGE_RATE_PROVIDER %q (expected fixed or vietcombank)", providerMode)
	}
	serviceLogger.Info("admin rates provider loaded", "provider", providerMode)
	handler := adminhttp.New(application.NewGetSystemRates(ratesProvider), serviceLogger, cfg.ServiceName)
	if err := httpx.Run(serviceLogger, cfg.Port, handler); err != nil {
		serviceLogger.Error("service stopped with error", "error", err)
		log.Fatal(err)
	}
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
