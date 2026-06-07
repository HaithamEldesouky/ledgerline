// Command ledgerline starts the LedgerLine HTTP service.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/haithamEldesouky/ledgerline/internal/config"
	"github.com/haithamEldesouky/ledgerline/internal/domain"
	"github.com/haithamEldesouky/ledgerline/internal/httpapi"
	"github.com/haithamEldesouky/ledgerline/internal/observability"
	"github.com/haithamEldesouky/ledgerline/internal/storage/memory"
	"github.com/haithamEldesouky/ledgerline/internal/storage/postgres"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := observability.NewLogger(cfg.Env, cfg.LogLevel)

	repo, err := newRepository(cfg, logger)
	if err != nil {
		return fmt.Errorf("initialise storage: %w", err)
	}
	defer func() { _ = repo.Close() }()

	svc := domain.NewService(repo)
	metrics := httpapi.NewMetrics()
	limiter := httpapi.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)
	defer limiter.Close()

	handler := httpapi.NewRouter(httpapi.RouterDeps{
		Service:     svc,
		Repository:  repo,
		Logger:      logger,
		Metrics:     metrics,
		RateLimiter: limiter,
	})

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting server",
			slog.String("version", version),
			slog.String("addr", addr),
			slog.String("driver", cfg.StorageDriver),
			slog.String("env", cfg.Env),
		)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErr:
		return fmt.Errorf("server failed: %w", err)
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining connections")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}
		logger.Info("shutdown complete")
	}
	return nil
}

func newRepository(cfg config.Config, logger *slog.Logger) (domain.Repository, error) {
	if cfg.StorageDriver == "postgres" {
		logger.Info("using postgres storage")
		repo, err := postgres.New(context.Background(), cfg.DatabaseURL)
		if err != nil {
			return nil, err
		}
		return repo, nil
	}
	logger.Info("using in-memory storage")
	return memory.New(), nil
}
