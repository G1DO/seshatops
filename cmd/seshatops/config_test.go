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

func TestConfigRejectsInvalidListenAddr(t *testing.T) {
	for _, bad := range []string{"localhost", ":99999", "127.0.0.1:", "127.0.0.1:0", "127.0.0.1:abc"} {
		cfg := validConfig()
		cfg.ListenAddr = bad
		if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), envListenAddr) {
			t.Fatalf("listen %q: error = %v", bad, err)
		}
	}
}

func TestConfigRejectsInvalidDatabaseURL(t *testing.T) {
	cases := []string{
		"http://user@localhost/db",
		"postgres://localhost/db",
		"postgres:///db",
		"postgres://user@localhost/db#frag",
		"postgres://user@",
	}
	for _, bad := range cases {
		cfg := validConfig()
		cfg.DatabaseURL = bad
		if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), envDatabaseURL) {
			t.Fatalf("db %q: error = %v", bad, err)
		}
	}
}

func TestConfigRejectsInvalidIssuer(t *testing.T) {
	cases := []string{
		"ftp://issuer.example",
		"http://issuer.example?query=1",
		"http://issuer.example#frag",
		"http://user:pass@issuer.example",
		"not-a-url",
	}
	for _, bad := range cases {
		cfg := validConfig()
		cfg.OIDCIssuer = bad
		if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), envOIDCIssuer) {
			t.Fatalf("issuer %q: error = %v", bad, err)
		}
	}
}

func TestConfigRejectsInvalidRedirectURL(t *testing.T) {
	cases := []string{
		"http://app.example/other",
		"http://app.example/auth/callback?x=1",
		"http://app.example/auth/callback#frag",
		"http://app.example",
		"not-a-url",
	}
	for _, bad := range cases {
		cfg := validConfig()
		cfg.OIDCRedirectURL = bad
		if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), envOIDCRedirectURL) {
			t.Fatalf("redirect %q: error = %v", bad, err)
		}
	}
}

func TestConfigRejectsNonPositiveSessionTTL(t *testing.T) {
	env := validEnv()
	env[envSessionTTL] = "0s"
	if _, err := configFromEnv(mapLookup(env)); err == nil || !strings.Contains(err.Error(), envSessionTTL) {
		t.Fatalf("session ttl 0: error = %v", err)
	}
	env[envSessionTTL] = "-1h"
	if _, err := configFromEnv(mapLookup(env)); err == nil || !strings.Contains(err.Error(), envSessionTTL) {
		t.Fatalf("session ttl negative: error = %v", err)
	}
	env[envSessionTTL] = "not-a-duration"
	if _, err := configFromEnv(mapLookup(env)); err == nil || !strings.Contains(err.Error(), envSessionTTL) {
		t.Fatalf("session ttl malformed: error = %v", err)
	}
	cfg := validConfig()
	cfg.SessionTTL = 0
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), envSessionTTL) {
		t.Fatalf("validate session ttl 0: error = %v", err)
	}
}

func TestConfigRejectsInvalidRetryAndShutdownBounds(t *testing.T) {
	cfg := validConfig()
	cfg.RelayInterval = 0
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "relay interval") {
		t.Fatalf("relay interval 0: error = %v", err)
	}
	cfg = validConfig()
	cfg.PollTimeout = 0
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "poll timeout") {
		t.Fatalf("poll timeout 0: error = %v", err)
	}
	cfg = validConfig()
	cfg.CycleTimeout = 0
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "cycle timeout") {
		t.Fatalf("cycle timeout 0: error = %v", err)
	}
	cfg = validConfig()
	cfg.RetryBase = 0
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "retry") {
		t.Fatalf("retry base 0: error = %v", err)
	}
	cfg = validConfig()
	cfg.RetryMax = time.Millisecond
	cfg.RetryBase = time.Second
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "retry") {
		t.Fatalf("retry max < base: error = %v", err)
	}
	cfg = validConfig()
	cfg.Shutdown = 0
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "shutdown") {
		t.Fatalf("shutdown 0: error = %v", err)
	}
}

func TestConfigRejectsMalformedAssignments(t *testing.T) {
	cases := []string{
		"operator|tenant",
		"operator||ROLE-OPS",
		"|tenant|role",
		"operator|tenant|",
		"operator|tenant|role|extra",
	}
	for _, bad := range cases {
		if _, err := parseAssignments(bad); err == nil {
			t.Fatalf("assignment %q accepted", bad)
		}
	}
	env := validEnv()
	env[envAssignments] = "bad-format"
	if _, err := configFromEnv(mapLookup(env)); err == nil || !strings.Contains(err.Error(), envAssignments) {
		t.Fatalf("assignments bad-format: error = %v", err)
	}
}

func TestConfigRejectsEmptyOrDuplicateBrokerSeeds(t *testing.T) {
	env := validEnv()
	env[envBrokerSeeds] = "localhost:9092, localhost:9092"
	if _, err := configFromEnv(mapLookup(env)); err == nil || !strings.Contains(err.Error(), envBrokerSeeds) {
		t.Fatalf("duplicate seeds: error = %v", err)
	}
	env[envBrokerSeeds] = "localhost:9092,,localhost:9093"
	if _, err := configFromEnv(mapLookup(env)); err == nil || !strings.Contains(err.Error(), envBrokerSeeds) {
		t.Fatalf("empty seed: error = %v", err)
	}
	cfg := validConfig()
	cfg.BrokerSeeds = []string{}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), envBrokerSeeds) {
		t.Fatalf("empty seeds validate: error = %v", err)
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
