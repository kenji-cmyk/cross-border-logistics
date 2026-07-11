package main

import (
	"log"
	"os"
	"time"

	adminconfig "github.com/example/cross-border-logistics/internal/admin/adapters/config"
	adminhttp "github.com/example/cross-border-logistics/internal/admin/adapters/http"
	"github.com/example/cross-border-logistics/internal/admin/application"
	"github.com/example/cross-border-logistics/pkg/config"
	"github.com/example/cross-border-logistics/pkg/httpx"
	"github.com/example/cross-border-logistics/pkg/logger"
)

func main() {
	cfg, err := config.Load("admin-service", "ADMIN_SERVICE_PORT", "ADMIN_DB")
	if err != nil {
		log.Fatal(err)
	}

	serviceLogger := logger.New(cfg.ServiceName, cfg.AppEnv)
	ratesProvider, err := adminconfig.Load(os.LookupEnv, time.Now())
	if err != nil {
		serviceLogger.Error("admin rates configuration failed", "error", err)
		log.Fatal(err)
	}
	serviceLogger.Info("admin rates configuration loaded")
	handler := adminhttp.New(application.NewGetSystemRates(ratesProvider), serviceLogger, cfg.ServiceName)
	if err := httpx.Run(serviceLogger, cfg.Port, handler); err != nil {
		serviceLogger.Error("service stopped with error", "error", err)
		log.Fatal(err)
	}
}
