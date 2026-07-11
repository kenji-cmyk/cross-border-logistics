package main

import (
	"context"
	"log"
	"os/signal"
	"sync"
	"syscall"
	"time"

	warehousehttp "github.com/example/cross-border-logistics/internal/warehouse/adapters/http"
	"github.com/example/cross-border-logistics/internal/warehouse/adapters/orderclient"
	warehousepostgres "github.com/example/cross-border-logistics/internal/warehouse/adapters/postgres"
	"github.com/example/cross-border-logistics/internal/warehouse/application"
	"github.com/example/cross-border-logistics/pkg/config"
	"github.com/example/cross-border-logistics/pkg/httpx"
	sharedkafka "github.com/example/cross-border-logistics/pkg/kafka"
	"github.com/example/cross-border-logistics/pkg/logger"
	sharedpostgres "github.com/example/cross-border-logistics/pkg/postgres"
)

func main() {
	cfg, err := config.Load("warehouse-service", "WAREHOUSE_SERVICE_PORT", "WAREHOUSE_DB")
	if err != nil {
		log.Fatal(err)
	}

	serviceLogger := logger.New(cfg.ServiceName, cfg.AppEnv)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	pool, err := sharedpostgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		serviceLogger.Error("database startup failed", "error", err)
		log.Fatal(err)
	}
	defer pool.Close()
	if err := warehousepostgres.Migrate(ctx, pool); err != nil {
		serviceLogger.Error("migration failed", "error", err)
		log.Fatal(err)
	}
	repository := warehousepostgres.New(pool)
	orders := orderclient.New(cfg.OrderServiceURL, cfg.HTTPClientTimeout, serviceLogger)
	service := application.NewService(repository, orders)
	producer, err := sharedkafka.NewProducer(cfg.KafkaBrokers)
	if err != nil {
		serviceLogger.Error("Kafka producer startup failed", "error", err)
		return
	}
	defer producer.Close()
	if err := sharedkafka.WaitReady(ctx, producer, 5, time.Second); err != nil {
		serviceLogger.Error("Kafka unavailable", "error", err)
		return
	}
	worker := sharedkafka.NewOutboxWorker(repository, producer, cfg.OutboxPollInterval, serviceLogger)
	var workers sync.WaitGroup
	workers.Add(1)
	go func() { defer workers.Done(); worker.Run(ctx) }()
	if err := httpx.RunContext(ctx, serviceLogger, cfg.Port, warehousehttp.New(service, serviceLogger, cfg.ServiceName)); err != nil {
		serviceLogger.Error("service stopped with error", "error", err)
	}
	stop()
	workers.Wait()
}
