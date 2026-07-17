package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

const shutdownTimeout = 10 * time.Second

type Health struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

func Run(logger *slog.Logger, port string, handler http.Handler) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return RunContext(ctx, logger, port, handler)
}

func RunContext(ctx context.Context, logger *slog.Logger, port string, handler http.Handler) error {
	return runContext(ctx, logger, port, handler, 10*time.Second)
}

// RunStreamingContext disables the absolute response write deadline so
// long-lived handlers such as Server-Sent Events can keep flushing data.
func RunStreamingContext(ctx context.Context, logger *slog.Logger, port string, handler http.Handler) error {
	return runContext(ctx, logger, port, handler, 0)
}

func runContext(ctx context.Context, logger *slog.Logger, port string, handler http.Handler, writeTimeout time.Duration) error {
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           RequestIDMiddleware(LoggingMiddleware(logger, handler)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       60 * time.Second,
	}

	serveErrors := make(chan error, 1)
	go func() {
		logger.Info("http server starting", "address", server.Addr)
		serveErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serveErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	if err := <-serveErrors; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	logger.Info("http server stopped")
	return nil
}
