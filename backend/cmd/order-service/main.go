package main

import (
	"context"
	"log"
	"os/signal"
	"sync"
	"syscall"
	"time"

	orderhttp "github.com/kenji-cmyk/cross-border-logistics/internal/order/adapters/http"
	orderkafka "github.com/kenji-cmyk/cross-border-logistics/internal/order/adapters/kafka"
	orderpostgres "github.com/kenji-cmyk/cross-border-logistics/internal/order/adapters/postgres"
	"github.com/kenji-cmyk/cross-border-logistics/internal/order/adapters/quotationclient"
	"github.com/kenji-cmyk/cross-border-logistics/internal/order/application"
	"github.com/kenji-cmyk/cross-border-logistics/pkg/config"
	"github.com/kenji-cmyk/cross-border-logistics/pkg/httpx"
	sharedkafka "github.com/kenji-cmyk/cross-border-logistics/pkg/kafka"
	"github.com/kenji-cmyk/cross-border-logistics/pkg/logger"
	sharedpostgres "github.com/kenji-cmyk/cross-border-logistics/pkg/postgres"
)

func main() {
	cfg, err := config.Load("order-service", "ORDER_SERVICE_PORT", "ORDER_DB")
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
	if err := orderpostgres.Migrate(ctx, pool); err != nil {
		serviceLogger.Error("migration failed", "error", err)
		log.Fatal(err)
	}
	repository := orderpostgres.New(pool)
	quotationReader := quotationclient.New(cfg.QuotationServiceURL, cfg.HTTPClientTimeout)
	service := application.NewService(repository, quotationReader)
	producer, err := sharedkafka.NewProducer(cfg.KafkaBrokers)
	if err != nil {
		serviceLogger.Error("Kafka producer startup failed", "error", err)
		return
	}
	defer producer.Close()
	consumer, err := orderkafka.NewConsumer(cfg.KafkaBrokers, "order-service-payment-events", service, serviceLogger)
	if err != nil {
		serviceLogger.Error("Kafka consumer startup failed", "error", err)
		return
	}
	defer consumer.Close()
	warehouseConsumer, err := orderkafka.NewWarehouseConsumer(cfg.KafkaBrokers, "order-service-warehouse-events", service, serviceLogger)
	if err != nil {
		serviceLogger.Error("warehouse Kafka consumer startup failed", "error", err)
		return
	}
	defer warehouseConsumer.Close()
	if err := sharedkafka.WaitReady(ctx, producer, 5, time.Second); err != nil {
		serviceLogger.Error("Kafka producer unavailable", "error", err)
		return
	}
	if err := sharedkafka.WaitReady(ctx, consumer, 5, time.Second); err != nil {
		serviceLogger.Error("Kafka consumer unavailable", "error", err)
		return
	}
	if err := sharedkafka.WaitReady(ctx, warehouseConsumer, 5, time.Second); err != nil {
		serviceLogger.Error("warehouse Kafka consumer unavailable", "error", err)
		return
	}
	handler := orderhttp.New(service, serviceLogger, cfg.ServiceName)
	outboxWorker := sharedkafka.NewOutboxWorker(repository, producer, cfg.OutboxPollInterval, serviceLogger)
	var workers sync.WaitGroup
	workers.Add(3)
	go func() { defer workers.Done(); outboxWorker.Run(ctx) }()
	go func() { defer workers.Done(); consumer.Run(ctx) }()
	go func() { defer workers.Done(); warehouseConsumer.Run(ctx) }()
	if err := httpx.RunContext(ctx, serviceLogger, cfg.Port, handler); err != nil {
		serviceLogger.Error("service stopped with error", "error", err)
	}
	stop()
	workers.Wait()
}
