package main

import (
	"context"
	"log"

	paymenthttp "github.com/example/cross-border-logistics/internal/payment/adapters/http"
	"github.com/example/cross-border-logistics/internal/payment/adapters/orderclient"
	paymentpostgres "github.com/example/cross-border-logistics/internal/payment/adapters/postgres"
	"github.com/example/cross-border-logistics/internal/payment/application"
	"github.com/example/cross-border-logistics/pkg/config"
	"github.com/example/cross-border-logistics/pkg/httpx"
	"github.com/example/cross-border-logistics/pkg/logger"
	sharedpostgres "github.com/example/cross-border-logistics/pkg/postgres"
)

func main() {
	cfg, err := config.Load("payment-service", "PAYMENT_SERVICE_PORT", "PAYMENT_DB")
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
	if err := paymentpostgres.Migrate(ctx, pool); err != nil {
		serviceLogger.Error("migration failed", "error", err)
		log.Fatal(err)
	}
	repository := paymentpostgres.New(pool)
	orderReader := orderclient.New(cfg.OrderServiceURL, cfg.HTTPClientTimeout)
	service := application.NewService(repository, orderReader)
	handler := paymenthttp.New(service, serviceLogger, cfg.ServiceName)
	if err := httpx.Run(serviceLogger, cfg.Port, handler); err != nil {
		serviceLogger.Error("service stopped with error", "error", err)
		log.Fatal(err)
	}
}
