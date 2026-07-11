package main

import (
	"context"
	"log"
	"os/signal"
	"sync"
	"syscall"
	"time"

	paymenthttp "github.com/example/cross-border-logistics/internal/payment/adapters/http"
	"github.com/example/cross-border-logistics/internal/payment/adapters/orderclient"
	paymentpostgres "github.com/example/cross-border-logistics/internal/payment/adapters/postgres"
	"github.com/example/cross-border-logistics/internal/payment/application"
	"github.com/example/cross-border-logistics/pkg/config"
	"github.com/example/cross-border-logistics/pkg/httpx"
	sharedkafka "github.com/example/cross-border-logistics/pkg/kafka"
	"github.com/example/cross-border-logistics/pkg/logger"
	sharedpostgres "github.com/example/cross-border-logistics/pkg/postgres"
)

func main() {
	cfg, err := config.Load("payment-service", "PAYMENT_SERVICE_PORT", "PAYMENT_DB")
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
	if err := paymentpostgres.Migrate(ctx, pool); err != nil {
		serviceLogger.Error("migration failed", "error", err)
		log.Fatal(err)
	}
	repository := paymentpostgres.New(pool)
	producer, err := sharedkafka.NewProducer(cfg.KafkaBrokers)
	if err != nil {
		serviceLogger.Error("Kafka producer startup failed", "error", err)
		return
	}
	defer producer.Close()
	if err := sharedkafka.WaitReady(ctx, producer, 5, time.Second); err != nil {
		serviceLogger.Error("Kafka startup failed", "error", err)
		return
	}
	orderReader := orderclient.New(cfg.OrderServiceURL, cfg.HTTPClientTimeout)
	service := application.NewService(repository, orderReader)
	handler := paymenthttp.New(service, serviceLogger, cfg.ServiceName)
	worker := sharedkafka.NewOutboxWorker(repository, producer, cfg.OutboxPollInterval, serviceLogger)
	var workers sync.WaitGroup
	workers.Add(1)
	go func() { defer workers.Done(); worker.Run(ctx) }()
	if err := httpx.RunContext(ctx, serviceLogger, cfg.Port, handler); err != nil {
		serviceLogger.Error("service stopped with error", "error", err)
	}
	stop()
	workers.Wait()
}
