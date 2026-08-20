package main

import (
	"strings"
	"testing"
	"time"
)

func TestConfigFromEnvValidatesRuntimeSettings(t *testing.T) {
	env := map[string]string{
		envListenAddr:      "127.0.0.1:8080",
		envDatabaseURL:     "postgres://seshatops@localhost/seshatops",
		envBrokerSeeds:     "localhost:9092,localhost:19092",
		envOIDCIssuer:      "http://issuer.example",
		envOIDCClientID:    "seshatops-ops",
		envOIDCRedirectURL: "http://app.example/auth/callback",
		envCookieName:      "seshatops_session",
		envCookieSecure:    "false",
		envSessionTTL:      "2h",
		envAssignments:     "operator|11111111-1111-4111-8111-111111111111|ROLE-OPS-READER",
	}

	cfg, err := configFromEnv(mapLookup(env))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SessionTTL != 2*time.Hour {
		t.Fatalf("session ttl = %s", cfg.SessionTTL)
	}
	if len(cfg.BrokerSeeds) != 2 || len(cfg.Assignments) != 1 {
		t.Fatalf("parsed config = %+v", cfg)
	}
	if cfg.Assignments[0].PrincipalID != "operator" {
		t.Fatalf("assignment = %+v", cfg.Assignments[0])
	}
}

func TestConfigFromEnvRejectsMissingRequiredDependency(t *testing.T) {
	env := validEnv()
	delete(env, envDatabaseURL)

	_, err := configFromEnv(mapLookup(env))
	if err == nil || !strings.Contains(err.Error(), envDatabaseURL) {
		t.Fatalf("error = %v", err)
	}
}

func TestConfigRejectsContradictoryCookieTransport(t *testing.T) {
	cfg := validConfig()
	cfg.CookieSecure = true

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), envCookieSecure) {
		t.Fatalf("error = %v", err)
	}
}

func TestConfigRejectsMalformedBrokerSeed(t *testing.T) {
	cfg := validConfig()
	cfg.BrokerSeeds = []string{"redpanda"}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), envBrokerSeeds) {
		t.Fatalf("error = %v", err)
	}
}

func TestConfigRejectsBlankAudienceOverride(t *testing.T) {
	env := validEnv()
	env[envOIDCAudience] = "   "

	_, err := configFromEnv(mapLookup(env))
	if err == nil || !strings.Contains(err.Error(), envOIDCAudience) {
		t.Fatalf("error = %v", err)
	}
}

func validEnv() map[string]string {
	return map[string]string{
		envListenAddr:      "127.0.0.1:8080",
		envDatabaseURL:     "postgres://seshatops@localhost/seshatops",
		envBrokerSeeds:     "localhost:9092",
		envOIDCIssuer:      "http://issuer.example",
		envOIDCClientID:    "seshatops-ops",
		envOIDCRedirectURL: "http://app.example/auth/callback",
		envCookieName:      "seshatops_session",
		envCookieSecure:    "false",
	}
}

func validConfig() Config {
	return Config{
		ListenAddr:      "127.0.0.1:8080",
		DatabaseURL:     "postgres://seshatops@localhost/seshatops",
		BrokerSeeds:     []string{"localhost:9092"},
		OIDCIssuer:      "http://issuer.example",
		OIDCClientID:    "seshatops-ops",
		OIDCRedirectURL: "http://app.example/auth/callback",
		CookieName:      "seshatops_session",
		SessionTTL:      time.Hour,
		RelayInterval:   time.Second,
		PollTimeout:     time.Second,
		CycleTimeout:    time.Second,
		RetryBase:       time.Second,
		RetryMax:        2 * time.Second,
		Shutdown:        time.Second,
	}
}

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}
