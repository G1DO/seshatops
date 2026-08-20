package main

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/G1DO/seshatops/identity"
)

const (
	envListenAddr       = "SESHATOPS_LISTEN_ADDR"
	envDatabaseURL      = "SESHATOPS_DATABASE_URL"
	envBrokerSeeds      = "SESHATOPS_BROKER_SEEDS"
	envOIDCIssuer       = "SESHATOPS_OIDC_ISSUER"
	envOIDCClientID     = "SESHATOPS_OIDC_CLIENT_ID"
	envOIDCClientSecret = "SESHATOPS_OIDC_CLIENT_SECRET"
	envOIDCAudience     = "SESHATOPS_OIDC_AUDIENCE"
	envOIDCRedirectURL  = "SESHATOPS_OIDC_REDIRECT_URL"
	envCookieName       = "SESHATOPS_COOKIE_NAME"
	envCookieSecure     = "SESHATOPS_COOKIE_SECURE"
	envSessionTTL       = "SESHATOPS_SESSION_TTL"
	envAssignments      = "SESHATOPS_AUTH_ASSIGNMENTS"
)

const (
	defaultSessionTTL    = 12 * time.Hour
	defaultRelayInterval = 500 * time.Millisecond
	defaultPollTimeout   = 1 * time.Second
	defaultCycleTimeout  = 10 * time.Second
	defaultRetryBase     = 1 * time.Second
	defaultRetryMax      = 30 * time.Second
	defaultShutdown      = 10 * time.Second
	defaultStartup       = 2 * time.Minute
)

// Config is the process configuration. Secrets are retained only in memory
// for the clients that need them and are never included in validation errors.
type Config struct {
	ListenAddr       string
	DatabaseURL      string
	BrokerSeeds      []string
	OIDCIssuer       string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCAudience     string
	OIDCRedirectURL  string
	CookieName       string
	CookieSecure     bool
	SessionTTL       time.Duration
	Assignments      []identity.Assignment

	RelayInterval time.Duration
	PollTimeout   time.Duration
	CycleTimeout  time.Duration
	RetryBase     time.Duration
	RetryMax      time.Duration
	Shutdown      time.Duration
}

// LoadConfig reads the runtime environment and rejects incomplete or
// contradictory settings before any listener or dependency client is opened.
func LoadConfig() (Config, error) {
	return configFromEnv(os.LookupEnv)
}

func configFromEnv(lookup func(string) (string, bool)) (Config, error) {
	var cfg Config
	var err error
	if cfg.ListenAddr, err = requiredEnv(lookup, envListenAddr); err != nil {
		return Config{}, err
	}
	if cfg.DatabaseURL, err = requiredEnv(lookup, envDatabaseURL); err != nil {
		return Config{}, err
	}
	var rawSeeds string
	if rawSeeds, err = requiredEnv(lookup, envBrokerSeeds); err != nil {
		return Config{}, err
	}
	if cfg.BrokerSeeds, err = parseBrokerSeeds(rawSeeds); err != nil {
		return Config{}, fmt.Errorf("%s: %w", envBrokerSeeds, err)
	}
	if cfg.OIDCIssuer, err = requiredEnv(lookup, envOIDCIssuer); err != nil {
		return Config{}, err
	}
	if cfg.OIDCClientID, err = requiredEnv(lookup, envOIDCClientID); err != nil {
		return Config{}, err
	}
	cfg.OIDCClientSecret, _ = lookup(envOIDCClientSecret)
	if rawAudience, ok := lookup(envOIDCAudience); ok {
		cfg.OIDCAudience = strings.TrimSpace(rawAudience)
		if rawAudience != "" && cfg.OIDCAudience == "" {
			return Config{}, fmt.Errorf("%s must not be blank", envOIDCAudience)
		}
	}
	if cfg.OIDCRedirectURL, err = requiredEnv(lookup, envOIDCRedirectURL); err != nil {
		return Config{}, err
	}
	if cfg.CookieName, err = requiredEnv(lookup, envCookieName); err != nil {
		return Config{}, err
	}
	var rawSecure string
	if rawSecure, err = requiredEnv(lookup, envCookieSecure); err != nil {
		return Config{}, err
	}
	if cfg.CookieSecure, err = strconv.ParseBool(rawSecure); err != nil {
		return Config{}, fmt.Errorf("%s must be true or false", envCookieSecure)
	}

	cfg.SessionTTL = defaultSessionTTL
	if raw, ok := lookup(envSessionTTL); ok && strings.TrimSpace(raw) != "" {
		cfg.SessionTTL, err = time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("%s must be a positive duration", envSessionTTL)
		}
	}

	if raw, ok := lookup(envAssignments); ok {
		cfg.Assignments, err = parseAssignments(raw)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", envAssignments, err)
		}
	}
	cfg.RelayInterval = defaultRelayInterval
	cfg.PollTimeout = defaultPollTimeout
	cfg.CycleTimeout = defaultCycleTimeout
	cfg.RetryBase = defaultRetryBase
	cfg.RetryMax = defaultRetryMax
	cfg.Shutdown = defaultShutdown

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	var problems []string
	if strings.TrimSpace(c.ListenAddr) == "" {
		problems = append(problems, envListenAddr+" is required")
	} else if err := validateListenAddr(c.ListenAddr); err != nil {
		problems = append(problems, envListenAddr+": "+err.Error())
	}
	if strings.TrimSpace(c.DatabaseURL) == "" {
		problems = append(problems, envDatabaseURL+" is required")
	} else if err := validateDatabaseURL(c.DatabaseURL); err != nil {
		problems = append(problems, envDatabaseURL+": "+err.Error())
	}
	if err := validateBrokerSeeds(c.BrokerSeeds); err != nil {
		problems = append(problems, envBrokerSeeds+": "+err.Error())
	}
	if strings.TrimSpace(c.OIDCIssuer) == "" {
		problems = append(problems, envOIDCIssuer+" is required")
	} else if err := validateIssuer(c.OIDCIssuer); err != nil {
		problems = append(problems, envOIDCIssuer+": "+err.Error())
	}
	if strings.TrimSpace(c.OIDCClientID) == "" {
		problems = append(problems, envOIDCClientID+" is required")
	}
	if c.OIDCAudience != "" && strings.TrimSpace(c.OIDCAudience) == "" {
		problems = append(problems, envOIDCAudience+" must not be blank")
	}
	if strings.TrimSpace(c.OIDCRedirectURL) == "" {
		problems = append(problems, envOIDCRedirectURL+" is required")
	} else if err := validateRedirectURL(c.OIDCRedirectURL); err != nil {
		problems = append(problems, envOIDCRedirectURL+": "+err.Error())
	}
	if strings.TrimSpace(c.CookieName) == "" {
		problems = append(problems, envCookieName+" is required")
	} else if err := (&http.Cookie{Name: c.CookieName}).Valid(); err != nil {
		problems = append(problems, envCookieName+": invalid cookie name")
	}
	if redirect, err := url.Parse(c.OIDCRedirectURL); err == nil {
		if redirect.Scheme == "https" && !c.CookieSecure {
			problems = append(problems, envCookieSecure+" must be true when the OIDC redirect URL uses https")
		}
		if redirect.Scheme == "http" && c.CookieSecure {
			problems = append(problems, envCookieSecure+" must be false when the OIDC redirect URL uses http")
		}
	}
	if c.SessionTTL <= 0 {
		problems = append(problems, envSessionTTL+" must be positive")
	}
	if c.RelayInterval <= 0 {
		problems = append(problems, "relay interval must be positive")
	}
	if c.PollTimeout <= 0 {
		problems = append(problems, "consumer poll timeout must be positive")
	}
	if c.CycleTimeout <= 0 {
		problems = append(problems, "worker cycle timeout must be positive")
	}
	if c.RetryBase <= 0 || c.RetryMax < c.RetryBase {
		problems = append(problems, "worker retry bounds are invalid")
	}
	if c.Shutdown <= 0 {
		problems = append(problems, "shutdown timeout must be positive")
	}
	if len(problems) > 0 {
		return fmt.Errorf("invalid runtime configuration: %s", strings.Join(problems, "; "))
	}
	return nil
}

func requiredEnv(lookup func(string) (string, bool), name string) (string, error) {
	value, ok := lookup(name)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return strings.TrimSpace(value), nil
}

func validateListenAddr(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		return fmt.Errorf("must be host:port")
	}
	if host == "" {
		// :port is a valid all-interface listen address.
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

func validateDatabaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return fmt.Errorf("must be a postgres:// or postgresql:// URL")
	}
	if u.User == nil || u.User.Username() == "" || u.Hostname() == "" {
		return fmt.Errorf("must include a database host and user")
	}
	if u.Fragment != "" {
		return fmt.Errorf("must not include a fragment")
	}
	return nil
}

func parseBrokerSeeds(raw string) ([]string, error) {
	parts := strings.Split(raw, ",")
	seeds := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		seed := strings.TrimSpace(part)
		if seed == "" {
			return nil, fmt.Errorf("contains an empty broker seed")
		}
		if _, ok := seen[seed]; ok {
			return nil, fmt.Errorf("contains duplicate broker seed")
		}
		if err := validateBrokerSeed(seed); err != nil {
			return nil, err
		}
		seen[seed] = struct{}{}
		seeds = append(seeds, seed)
	}
	return seeds, nil
}

func validateBrokerSeeds(seeds []string) error {
	if len(seeds) == 0 {
		return fmt.Errorf("at least one seed is required")
	}
	seen := make(map[string]struct{}, len(seeds))
	for _, seed := range seeds {
		if _, ok := seen[seed]; ok {
			return fmt.Errorf("contains duplicate broker seed")
		}
		if err := validateBrokerSeed(seed); err != nil {
			return err
		}
		seen[seed] = struct{}{}
	}
	return nil
}

func validateBrokerSeed(seed string) error {
	host, port, err := net.SplitHostPort(seed)
	if err != nil || host == "" || port == "" {
		return fmt.Errorf("must be host:port")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("has an invalid port")
	}
	return nil
}

func validateIssuer(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("must be an http or https URL")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("must not include credentials, query, or fragment")
	}
	return nil
}

func validateRedirectURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("must be an absolute http or https URL")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "/auth/callback" {
		return fmt.Errorf("must be a credential-free URL with path /auth/callback")
	}
	return nil
}

func parseAssignments(raw string) ([]identity.Assignment, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]identity.Assignment, 0, len(parts))
	for _, part := range parts {
		fields := strings.Split(part, "|")
		if len(fields) != 3 {
			return nil, fmt.Errorf("assignment must be principal|tenant|role")
		}
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
			if fields[i] == "" {
				return nil, fmt.Errorf("assignment contains an empty field")
			}
		}
		out = append(out, identity.Assignment{
			PrincipalID: fields[0],
			TenantID:    fields[1],
			RoleID:      fields[2],
		})
	}
	return out, nil
}
