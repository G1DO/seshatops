package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "forecast":
			cfg, err := loadForecastCommandConfig()
			if err != nil {
				processFailure("forecast.configuration_failed")
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if err := runForecastCommand(ctx, cfg, os.Stdout); err != nil {
				processFailure("forecast.command_failed")
			}
			return
		case "bootstrap":
			cfg, err := loadBootstrapCommandConfig()
			if err != nil {
				processFailure("bootstrap.configuration_failed")
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if err := runBootstrapCommand(ctx, cfg, os.Stdout); err != nil {
				processFailure("bootstrap.command_failed")
			}
			return
		case "reset-northstar":
			databaseURL, err := loadResetCommandConfig()
			if err != nil {
				processFailure("reset.configuration_failed")
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if err := runResetNorthstarCommand(ctx, databaseURL); err != nil {
				processFailure("reset.command_failed")
			}
			return
		default:
			processFailure("command.unknown")
		}
	}

	cfg, err := LoadConfig()
	if err != nil {
		processFailure("runtime.configuration_failed")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	r, err := newRuntime(ctx, cfg)
	if err != nil {
		processFailure("runtime.initialization_failed")
	}
	if err := r.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		processFailure("runtime.failed")
	}
}

func processFailure(event string) {
	slog.Error(event)
	os.Exit(1)
}
