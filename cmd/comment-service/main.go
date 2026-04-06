package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Matthew11K/Comments-Service/internal/config"
	"github.com/Matthew11K/Comments-Service/internal/di"
	"github.com/Matthew11K/Comments-Service/internal/logging"
)

func main() {
	if err := run(); err != nil {
		slog.Error("comment-service exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		return err
	}

	logger, err := logging.New(cfg.Logging, os.Stdout)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	container, err := di.NewContainer(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
		defer cancel()
		if err := container.Close(shutdownCtx); err != nil {
			logger.Error("close container", "error", err)
		}
	}()

	if err := container.Build(); err != nil {
		return err
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting http server", "addr", cfg.HTTP.Addr, "backend", cfg.Storage.Backend)
		if err := container.HTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case <-ctx.Done():
	case err := <-serverErr:
		if err != nil {
			return err
		}
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()

	logger.Info("shutting down http server")
	if err := container.Shutdown(shutdownCtx); err != nil {
		return err
	}

	return nil
}
