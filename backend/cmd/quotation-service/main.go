package main

import (
	"context"
	"log"

	"github.com/example/cross-border-logistics/internal/quotation/adapters"
	quotationhttp "github.com/example/cross-border-logistics/internal/quotation/adapters/http"
	quotationpostgres "github.com/example/cross-border-logistics/internal/quotation/adapters/postgres"
	"github.com/example/cross-border-logistics/internal/quotation/application"
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
	service := application.NewService(repository, adapters.MockExchangeRates{}, adapters.MockRestrictionChecker{})
	handler := quotationhttp.New(service, serviceLogger, cfg.ServiceName)
	if err := httpx.Run(serviceLogger, cfg.Port, handler); err != nil {
		serviceLogger.Error("service stopped with error", "error", err)
		log.Fatal(err)
	}
}
