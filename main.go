package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"archery-auto-approve/api"
	"archery-auto-approve/config"
	"archery-auto-approve/scheduler"
	"archery-auto-approve/utils"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config failed: %v\n", err)
		os.Exit(1)
	}

	logger, err := utils.NewLogger(cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init logger failed: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = logger.Sync()
	}()

	rootCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	client := api.NewClient(cfg, logger)
	svc := scheduler.New(cfg, client, logger)

	server := buildHealthServer(cfg)

	if cfg.Health.Enabled {
		go func() {
			logger.Info("health server started", utils.FieldString("addr", server.Addr))
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Error("health server exited with error", utils.FieldError(err))
				cancel()
			}
		}()
	}

	go func() {
		if err := svc.Run(rootCtx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("scheduler exited with error", utils.FieldError(err))
			cancel()
		}
	}()

	<-rootCtx.Done()
	logger.Info("shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if cfg.Health.Enabled {
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Warn("health server shutdown failed", utils.FieldError(err))
		}
	}

	logger.Info("service stopped")
}

func buildHealthServer(cfg *config.Config) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	return &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Health.Port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
}
