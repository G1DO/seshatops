package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "bootstrap":
			cfg, err := loadBootstrapCommandConfig()
			if err != nil {
				log.Fatal(err)
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if err := runBootstrapCommand(ctx, cfg, os.Stdout); err != nil {
				log.Fatal(err)
			}
			return
		case "reset-northstar":
			databaseURL, err := loadResetCommandConfig()
			if err != nil {
				log.Fatal(err)
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if err := runResetNorthstarCommand(ctx, databaseURL); err != nil {
				log.Fatal(err)
			}
			return
		default:
			log.Fatalf("unknown command %q; use bootstrap or reset-northstar", os.Args[1])
		}
	}

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	r, err := newRuntime(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := r.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
