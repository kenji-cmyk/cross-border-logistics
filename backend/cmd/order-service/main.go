package main

import (
	"context"
	"log"

	orderhttp "github.com/example/cross-border-logistics/internal/order/adapters/http"
	orderpostgres "github.com/example/cross-border-logistics/internal/order/adapters/postgres"
	"github.com/example/cross-border-logistics/internal/order/adapters/quotationclient"
	"github.com/example/cross-border-logistics/internal/order/application"
	"github.com/example/cross-border-logistics/pkg/config"
	"github.com/example/cross-border-logistics/pkg/httpx"
	"github.com/example/cross-border-logistics/pkg/logger"
	sharedpostgres "github.com/example/cross-border-logistics/pkg/postgres"
)

func main() {
	cfg, err := config.Load("order-service", "ORDER_SERVICE_PORT", "ORDER_DB")
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
	if err := orderpostgres.Migrate(ctx, pool); err != nil {
		serviceLogger.Error("migration failed", "error", err)
		log.Fatal(err)
	}
	repository := orderpostgres.New(pool)
	quotationReader := quotationclient.New(cfg.QuotationServiceURL, cfg.HTTPClientTimeout)
	service := application.NewService(repository, quotationReader)
	handler := orderhttp.New(service, serviceLogger, cfg.ServiceName)
	if err := httpx.Run(serviceLogger, cfg.Port, handler); err != nil {
		serviceLogger.Error("service stopped with error", "error", err)
		log.Fatal(err)
	}
}
