package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	paymentadapters "github.com/example/cross-border-logistics/internal/payment/adapters"
	paymenthttp "github.com/example/cross-border-logistics/internal/payment/adapters/http"
	"github.com/example/cross-border-logistics/internal/payment/adapters/momo"
	"github.com/example/cross-border-logistics/internal/payment/adapters/orderclient"
	paymentpostgres "github.com/example/cross-border-logistics/internal/payment/adapters/postgres"
	"github.com/example/cross-border-logistics/internal/payment/application"
	"github.com/example/cross-border-logistics/internal/payment/ports"
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
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("PAYMENT_PROVIDER")))
	if provider == "" {
		provider = "mock"
	}
	var gateway ports.PaymentGateway
	var handler http.Handler
	var service *application.Service
	if provider == "momo" {
		client, e := momo.New(momo.Config{PartnerCode: os.Getenv("MOMO_PARTNER_CODE"), AccessKey: os.Getenv("MOMO_ACCESS_KEY"), SecretKey: os.Getenv("MOMO_SECRET_KEY"), BaseURL: os.Getenv("MOMO_API_BASE_URL"), IPNURL: os.Getenv("MOMO_IPN_URL"), RedirectURL: os.Getenv("MOMO_REDIRECT_URL"), Timeout: cfg.HTTPClientTimeout})
		if e != nil {
			log.Fatal(e)
		}
		gateway = client
		service = application.NewService(repository, orderReader, gateway)
		handler = paymenthttp.NewMoMo(service, client, serviceLogger, cfg.ServiceName, cfg.AppEnv)
	} else {
		webhookSecret := strings.TrimSpace(os.Getenv("PAYMENT_WEBHOOK_SECRET"))
		if strings.EqualFold(cfg.AppEnv, "production") && webhookSecret == "" {
			log.Fatal("PAYMENT_WEBHOOK_SECRET is required for mock provider in production")
		}
		gateway = paymentadapters.MockHostedGateway{}
		service = application.NewService(repository, orderReader, gateway)
		handler = paymenthttp.New(service, serviceLogger, cfg.ServiceName, webhookSecret, cfg.AppEnv)
	}
	worker := sharedkafka.NewOutboxWorker(repository, producer, cfg.OutboxPollInterval, serviceLogger)
	var workers sync.WaitGroup
	workers.Add(1)
	go func() { defer workers.Done(); worker.Run(ctx) }()
	if provider == "momo" {
		workers.Add(1)
		go func() { defer workers.Done(); service.RunReconciliation(ctx, 15*time.Second) }()
	}
	if err := httpx.RunContext(ctx, serviceLogger, cfg.Port, handler); err != nil {
		serviceLogger.Error("service stopped with error", "error", err)
	}
	stop()
	workers.Wait()
}
