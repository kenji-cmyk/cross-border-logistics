package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	paymentadapters "github.com/example/cross-border-logistics/internal/payment/adapters"
	paymenthttp "github.com/example/cross-border-logistics/internal/payment/adapters/http"
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
	webhookSecret := strings.TrimSpace(os.Getenv("SEPAY_WEBHOOK_SECRET"))
	pgSecret := strings.TrimSpace(os.Getenv("SEPAY_PG_SECRET_KEY"))
	sePayAccountNumber := strings.TrimSpace(os.Getenv("SEPAY_ACCOUNT_NUMBER"))
	var gateway ports.PaymentGateway
	switch provider {
	case "sepay":
		gateway, err = paymentadapters.NewSePayGateway(paymentadapters.SePayGatewayConfig{
			BankCode:          os.Getenv("SEPAY_BANK_CODE"),
			AccountNumber:     sePayAccountNumber,
			AccountHolder:     os.Getenv("SEPAY_ACCOUNT_HOLDER"),
			PaymentCodePrefix: os.Getenv("SEPAY_PAYMENT_CODE_PREFIX"),
			QRBaseURL:         os.Getenv("SEPAY_QR_BASE_URL"),
		})
		if err != nil {
			log.Fatalf("invalid SePay configuration: %v", err)
		}
		if webhookSecret == "" {
			log.Fatal("SEPAY_WEBHOOK_SECRET is required when PAYMENT_PROVIDER=sepay")
		}
	case "sepay_pg":
		gateway, err = paymentadapters.NewSePayPGGateway(paymentadapters.SePayPGGatewayConfig{
			Environment:   os.Getenv("SEPAY_PG_ENV"),
			MerchantID:    os.Getenv("SEPAY_PG_MERCHANT_ID"),
			SecretKey:     pgSecret,
			ReturnBaseURL: os.Getenv("SEPAY_PUBLIC_URL"),
		})
		if err != nil {
			log.Fatalf("invalid SePay Payment Gateway configuration: %v", err)
		}
	case "mock":
		if strings.EqualFold(cfg.AppEnv, "production") {
			log.Fatal("PAYMENT_PROVIDER=mock is not allowed in production")
		}
		gateway = paymentadapters.MockHostedGateway{}
	default:
		log.Fatalf("unsupported PAYMENT_PROVIDER %q (expected mock, sepay, or sepay_pg)", provider)
	}
	serviceLogger.Info("payment provider configured", "provider", provider)
	service := application.NewService(repository, orderReader, gateway)
	handler := paymenthttp.New(service, serviceLogger, cfg.ServiceName, webhookSecret, cfg.AppEnv, sePayAccountNumber, provider, pgSecret)
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
