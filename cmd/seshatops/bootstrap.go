package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/G1DO/seshatops/bootstrap"
	"github.com/G1DO/seshatops/erp"
	"github.com/G1DO/seshatops/identity"
	"github.com/G1DO/seshatops/northstar"
	"github.com/G1DO/seshatops/platform"
	"github.com/G1DO/seshatops/relay"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	envNorthstarBootstrapTimeout = "SESHATOPS_NORTHSTAR_BOOTSTRAP_TIMEOUT"
	envNorthstarResetConfirm     = "SESHATOPS_NORTHSTAR_RESET_CONFIRM"
	northstarResetConfirmation   = "I_UNDERSTAND_DISPOSABLE_NORTHSTAR_RESET"
	northstarDisposableDatabase  = "seshatops_northstar_disposable"
)

type bootstrapCommandConfig struct {
	DatabaseURL string
	BrokerSeeds []string
	Timeout     time.Duration
}

func loadBootstrapCommandConfig() (bootstrapCommandConfig, error) {
	return bootstrapCommandConfigFromEnv(os.LookupEnv)
}

func bootstrapCommandConfigFromEnv(lookup func(string) (string, bool)) (bootstrapCommandConfig, error) {
	databaseURL, err := requiredEnv(lookup, envDatabaseURL)
	if err != nil {
		return bootstrapCommandConfig{}, err
	}
	if err := validateDatabaseURL(databaseURL); err != nil {
		return bootstrapCommandConfig{}, fmt.Errorf("%s: %w", envDatabaseURL, err)
	}
	rawSeeds, err := requiredEnv(lookup, envBrokerSeeds)
	if err != nil {
		return bootstrapCommandConfig{}, err
	}
	seeds, err := parseBrokerSeeds(rawSeeds)
	if err != nil {
		return bootstrapCommandConfig{}, fmt.Errorf("%s: %w", envBrokerSeeds, err)
	}
	timeout := bootstrap.DefaultTimeout
	if raw, ok := lookup(envNorthstarBootstrapTimeout); ok && strings.TrimSpace(raw) != "" {
		timeout, err = time.ParseDuration(raw)
		if err != nil || timeout <= 0 {
			return bootstrapCommandConfig{}, fmt.Errorf("%s must be a positive duration", envNorthstarBootstrapTimeout)
		}
	}
	return bootstrapCommandConfig{DatabaseURL: databaseURL, BrokerSeeds: seeds, Timeout: timeout}, nil
}

func runBootstrapCommand(ctx context.Context, cfg bootstrapCommandConfig, out io.Writer) error {
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database failed; check %s", envDatabaseURL)
	}
	defer db.Close()

	startupCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	if err := db.PingContext(startupCtx); err != nil {
		return fmt.Errorf("database ping failed; check %s", envDatabaseURL)
	}
	if err := erp.Migrate(startupCtx, db); err != nil {
		return fmt.Errorf("ERP migration failed")
	}
	if err := platform.Migrate(startupCtx, db); err != nil {
		return fmt.Errorf("platform migration failed")
	}
	if err := identity.Migrate(startupCtx, db); err != nil {
		return fmt.Errorf("identity migration failed")
	}

	publisher, err := relay.NewFranzPublisher(cfg.BrokerSeeds...)
	if err != nil {
		return fmt.Errorf("create relay client failed; check %s", envBrokerSeeds)
	}
	defer publisher.Close()
	brokerCtx, brokerCancel := context.WithTimeout(ctx, cfg.Timeout)
	defer brokerCancel()
	if err := publisher.Ping(brokerCtx); err != nil {
		return fmt.Errorf("broker ping failed; check %s", envBrokerSeeds)
	}

	group, err := newBootstrapConsumerGroup()
	if err != nil {
		return fmt.Errorf("create bootstrap consumer group: %w", err)
	}
	consumer, err := platform.NewConsumerWithGroup(db, group, cfg.BrokerSeeds...)
	if err != nil {
		return fmt.Errorf("create consumer client failed; check %s", envBrokerSeeds)
	}
	defer consumer.Close()
	if err := consumer.Ping(brokerCtx); err != nil {
		return fmt.Errorf("consumer broker ping failed; check %s", envBrokerSeeds)
	}

	summary, err := bootstrap.Run(ctx, db, publisher, consumer, bootstrap.Config{
		Seed:          northstar.LineageSeed,
		Timeout:       cfg.Timeout,
		PollTimeout:   bootstrap.DefaultPollTimeout,
		RetryInterval: bootstrap.DefaultRetryInterval,
		RelayOwner:    group,
		LeaseTTL:      bootstrap.DefaultLeaseTTL,
		DrainLimit:    bootstrap.DefaultDrainLimit,
	})
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(summary)
}

func newBootstrapConsumerGroup() (string, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "seshatops-northstar-bootstrap-" + hex.EncodeToString(raw[:]), nil
}

func loadResetCommandConfig() (string, error) {
	databaseURL, err := requiredEnv(os.LookupEnv, envDatabaseURL)
	if err != nil {
		return "", err
	}
	if err := validateDisposableNorthstarURL(databaseURL); err != nil {
		return "", fmt.Errorf("%s: %w", envDatabaseURL, err)
	}
	confirmation, ok := os.LookupEnv(envNorthstarResetConfirm)
	if !ok || strings.TrimSpace(confirmation) != northstarResetConfirmation {
		return "", fmt.Errorf("%s must equal %s", envNorthstarResetConfirm, northstarResetConfirmation)
	}
	return databaseURL, nil
}

func validateDisposableNorthstarURL(raw string) error {
	if err := validateDatabaseURL(raw); err != nil {
		return err
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("must be a local disposable Northstar database")
	}
	host := strings.ToLower(u.Hostname())
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return fmt.Errorf("must target localhost")
	}
	if strings.TrimPrefix(u.Path, "/") != northstarDisposableDatabase {
		return fmt.Errorf("database name must be %s", northstarDisposableDatabase)
	}
	return nil
}

func runResetNorthstarCommand(ctx context.Context, databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open database failed; check %s", envDatabaseURL)
	}
	defer db.Close()
	resetCtx, cancel := context.WithTimeout(ctx, defaultStartup)
	defer cancel()
	if err := db.PingContext(resetCtx); err != nil {
		return fmt.Errorf("database ping failed; check %s", envDatabaseURL)
	}
	if err := erp.Migrate(resetCtx, db); err != nil {
		return fmt.Errorf("ERP migration failed")
	}
	if err := platform.Migrate(resetCtx, db); err != nil {
		return fmt.Errorf("platform migration failed")
	}
	if err := identity.Migrate(resetCtx, db); err != nil {
		return fmt.Errorf("identity migration failed")
	}
	if err := platform.ResetDerivedStateForTenant(resetCtx, db, identity.TenantNS001UUID); err != nil {
		return err
	}
	if err := erp.ResetSourceForTenant(resetCtx, db, identity.TenantNS001UUID); err != nil {
		return err
	}
	return nil
}
