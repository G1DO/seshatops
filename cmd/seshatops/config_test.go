package main

import (
	"strings"
	"testing"
	"time"

	"github.com/G1DO/seshatops/identity"
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

func TestConfigParsesGoSelectedReleaseScopeAssignment(t *testing.T) {
	assignments, err := parseAssignments("operator|SCOPE-RUNTIME|ROLE-RELEASE-OBSERVER")
	if err != nil {
		t.Fatal(err)
	}
	if len(assignments) != 1 || assignments[0].TenantID != identity.ScopeRuntime || assignments[0].RoleID != identity.RoleReleaseObserver {
		t.Fatalf("assignments=%+v", assignments)
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

func TestBootstrapCommandConfigUsesOnlyDatabaseAndBroker(t *testing.T) {
	env := map[string]string{
		envDatabaseURL:               "postgres://seshatops@localhost/seshatops",
		envBrokerSeeds:               "localhost:9092",
		envNorthstarBootstrapTimeout: "3s",
	}
	cfg, err := bootstrapCommandConfigFromEnv(mapLookup(env))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Timeout != 3*time.Second || len(cfg.BrokerSeeds) != 1 {
		t.Fatalf("bootstrap config = %+v", cfg)
	}
}

func TestBootstrapCommandConfigRejectsNonPositiveTimeout(t *testing.T) {
	env := map[string]string{
		envDatabaseURL:               "postgres://seshatops@localhost/seshatops",
		envBrokerSeeds:               "localhost:9092",
		envNorthstarBootstrapTimeout: "0s",
	}
	_, err := bootstrapCommandConfigFromEnv(mapLookup(env))
	if err == nil || !strings.Contains(err.Error(), envNorthstarBootstrapTimeout) {
		t.Fatalf("error = %v", err)
	}
}

func TestDisposableNorthstarResetTargetIsNarrowlyGated(t *testing.T) {
	valid := "postgres://seshatops@localhost/seshatops_northstar_disposable"
	if err := validateDisposableNorthstarURL(valid); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		"postgres://seshatops@localhost/seshatops",
		"postgres://seshatops@example.com/seshatops_northstar_disposable",
	} {
		if err := validateDisposableNorthstarURL(raw); err == nil {
			t.Fatalf("accepted unsafe reset target %q", raw)
		}
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
