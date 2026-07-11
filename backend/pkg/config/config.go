package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const defaultPort = "8080"

type Config struct {
	AppEnv              string
	ServiceName         string
	Port                string
	DatabaseURL         string
	KafkaBrokers        []string
	QuotationServiceURL string
	OrderServiceURL     string
	OutboxPollInterval  time.Duration
	HTTPClientTimeout   time.Duration
}

func Load(serviceName, portVariable, databaseVariable string) (Config, error) {
	outboxPollInterval, err := duration("OUTBOX_POLL_INTERVAL", time.Second)
	if err != nil {
		return Config{}, err
	}

	httpClientTimeout, err := duration("HTTP_CLIENT_TIMEOUT", 2*time.Second)
	if err != nil {
		return Config{}, err
	}

	return Config{
		AppEnv:              value("APP_ENV", "development"),
		ServiceName:         serviceName,
		Port:                value(portVariable, defaultPort),
		DatabaseURL:         databaseURL(databaseVariable),
		KafkaBrokers:        split(value("KAFKA_BROKERS", "kafka:9092")),
		QuotationServiceURL: value("QUOTATION_SERVICE_URL", "http://quotation-service:8080"),
		OrderServiceURL:     value("ORDER_SERVICE_URL", "http://order-service:8080"),
		OutboxPollInterval:  outboxPollInterval,
		HTTPClientTimeout:   httpClientTimeout,
	}, nil
}

func databaseURL(databaseVariable string) string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		value("POSTGRES_USER", "logistics"),
		value("POSTGRES_PASSWORD", "logistics"),
		value("POSTGRES_HOST", "postgres"),
		value("POSTGRES_PORT", "5432"),
		value(databaseVariable, "postgres"),
	)
}

func duration(name string, fallback time.Duration) (time.Duration, error) {
	raw := value(name, fallback.String())
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return parsed, nil
}

func value(name, fallback string) string {
	if current := strings.TrimSpace(os.Getenv(name)); current != "" {
		return current
	}
	return fallback
}

func split(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
