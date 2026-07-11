package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/example/cross-border-logistics/internal/quotation/adapters"
	"github.com/example/cross-border-logistics/internal/quotation/adapters/extraction"
	quotationhttp "github.com/example/cross-border-logistics/internal/quotation/adapters/http"
	quotationpostgres "github.com/example/cross-border-logistics/internal/quotation/adapters/postgres"
	"github.com/example/cross-border-logistics/internal/quotation/application"
	"github.com/example/cross-border-logistics/internal/quotation/ports"
	"github.com/example/cross-border-logistics/pkg/config"
	"github.com/example/cross-border-logistics/pkg/httpx"
	"github.com/example/cross-border-logistics/pkg/logger"
	sharedpostgres "github.com/example/cross-border-logistics/pkg/postgres"
)

func main() {
	cfg, err := config.Load("quotation-service", "QUOTATION_SERVICE_PORT", "QUOTATION_DB")
	if err != nil {
		log.Fatal(err)
	}

	serviceLogger := logger.New(cfg.ServiceName, cfg.AppEnv)
	ctx := context.Background()
	pool, err := sharedpostgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		serviceLogger.Error("database startup failed", "error", err)
		log.Fatal(err)
	}
	defer pool.Close()
	if err := quotationpostgres.Migrate(ctx, pool); err != nil {
		serviceLogger.Error("migration failed", "error", err)
		log.Fatal(err)
	}
	repository := quotationpostgres.New(pool)
	var rates ports.ExchangeRateProvider = adapters.MockExchangeRates{}
	if endpoint := os.Getenv("EXCHANGE_RATE_BASE_URL"); endpoint != "" {
		rates = adapters.NewHTTPExchangeRates(endpoint, 550*time.Millisecond)
	}
	extractionConfig, err := extraction.LoadConfig(os.Getenv)
	if err != nil {
		serviceLogger.Error("product extractor configuration is invalid", "error", err)
		log.Fatal(err)
	}
	var productExtractor ports.ProductExtractor = adapters.DemoProductExtractor{}
	if extractionConfig.Mode == extraction.ModeHybrid {
		hosts, err := extraction.NewHostMatcher(extractionConfig.AllowedHosts)
		if err != nil {
			serviceLogger.Error("product host configuration is invalid", "error", err)
			log.Fatal(err)
		}
		safety := extraction.NewURLSafetyValidator(hosts, nil)
		client := extraction.NewSafeHTTPClient(extractionConfig.FetchTimeout, extractionConfig.MaxRedirects, safety)
		httpExtractor := extraction.NewHTTPStructuredProductExtractor(client, safety, extractionConfig.MaxResponseBytes, serviceLogger)
		productExtractor = extraction.NewRoutingProductExtractor(adapters.DemoProductExtractor{}, httpExtractor, hosts)
	}
	service := application.NewService(repository, rates, adapters.MockRestrictionChecker{}, productExtractor)
	handler := quotationhttp.New(service, serviceLogger, cfg.ServiceName)
	if err := httpx.Run(serviceLogger, cfg.Port, handler); err != nil {
		serviceLogger.Error("service stopped with error", "error", err)
		log.Fatal(err)
	}
}
