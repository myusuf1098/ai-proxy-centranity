package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/myusuf1098/ai-proxy-centranity/internal/api"
	"github.com/myusuf1098/ai-proxy-centranity/internal/config"
	"github.com/myusuf1098/ai-proxy-centranity/internal/health"
	"github.com/myusuf1098/ai-proxy-centranity/internal/ninerouter"
	"github.com/myusuf1098/ai-proxy-centranity/internal/store"
)

func main() {
	// 1. Initialize structured JSON logger
	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(logHandler)
	slog.SetDefault(logger)

	logger.Info("starting ProxyGateway Enterprise",
		slog.String("version", "2.0"),
		slog.String("component", "api"),
	)

	// 2. Load Configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", slog.Any("error", err))
		os.Exit(1)
	}

	// 3. Initialize Stores
	var healthCheckers []health.HealthChecker

	pgStore, err := store.NewPostgresStore(cfg.Database)
	if err != nil {
		logger.Warn("postgres store initialization failed", slog.Any("error", err))
	} else {
		defer pgStore.Close()
		healthCheckers = append(healthCheckers, pgStore)

		// Check if migrations dir exists and apply migrations
		if _, err := os.Stat("migrations"); err == nil {
			migCtx, migCancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := store.RunMigrations(migCtx, pgStore.DB(), "migrations"); err != nil {
				logger.Warn("database migrations warning", slog.Any("error", err))
			} else {
				logger.Info("database migrations applied successfully")
			}
			migCancel()
		}
	}

	redisStore, err := store.NewRedisStore(cfg.Redis)
	if err != nil {
		logger.Warn("redis store initialization failed", slog.Any("error", err))
	} else {
		defer redisStore.Close()
		healthCheckers = append(healthCheckers, redisStore)
	}

	// 4. Initialize 9Router Upstream Adapter
	nineRouterAdapter := ninerouter.NewHTTPAdapter(ninerouter.Config{
		BaseURL: cfg.NineRouter.BaseURL,
		APIKey:  cfg.NineRouter.APIKey,
		Timeout: cfg.NineRouter.Timeout,
	})
	healthCheckers = append(healthCheckers, nineRouterAdapter)

	// 5. Setup Health Handlers & Router
	healthHandler := health.NewHandler(healthCheckers...)
	router := api.NewRouterWithAdapter(cfg, healthHandler, nineRouterAdapter, logger)

	// 6. Start Server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("HTTP server listening", slog.Int("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server failed", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	sig := <-stopChan
	logger.Info("shutdown signal received", slog.String("signal", sig.String()))

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("graceful server shutdown failed", slog.Any("error", err))
	} else {
		logger.Info("server stopped gracefully")
	}
}
