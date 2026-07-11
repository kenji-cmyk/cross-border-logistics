package main

import (
	"log"

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
	if err := httpx.Run(serviceLogger, cfg.Port, httpx.NewHealthHandler(cfg.ServiceName)); err != nil {
		serviceLogger.Error("service stopped with error", "error", err)
		log.Fatal(err)
	}
}
