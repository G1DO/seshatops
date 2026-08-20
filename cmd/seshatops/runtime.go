package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/G1DO/seshatops/api"
	"github.com/G1DO/seshatops/erp"
	"github.com/G1DO/seshatops/identity"
	"github.com/G1DO/seshatops/platform"
	"github.com/G1DO/seshatops/relay"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type readiness struct {
	mu     sync.RWMutex
	checks map[string]bool
}

func newReadiness(checks ...string) *readiness {
	state := &readiness{checks: make(map[string]bool, len(checks))}
	for _, check := range checks {
		state.checks[check] = false
	}
	return state
}

func (r *readiness) set(check string, healthy bool) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.checks[check] = healthy
	r.mu.Unlock()
}

func (r *readiness) ready() bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.checks) == 0 {
		return false
	}
	for _, healthy := range r.checks {
		if !healthy {
			return false
		}
	}
	return true
}

func (r *readiness) serveHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeHealthJSON(w, http.StatusMethodNotAllowed, "not_ready")
		return
	}
	if r.ready() {
		writeHealthJSON(w, http.StatusOK, "ready")
		return
	}
	writeHealthJSON(w, http.StatusServiceUnavailable, "not_ready")
}

func writeHealthJSON(w http.ResponseWriter, status int, value string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `{"status":%q}
`, value)
}

func composeHandler(authHandler, apiHandler http.Handler, state *readiness) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeHealthJSON(w, http.StatusMethodNotAllowed, "alive")
			return
		}
		writeHealthJSON(w, http.StatusOK, "alive")
	})
	mux.HandleFunc("/readyz", state.serveHTTP)
	if authHandler != nil {
		mux.Handle("/auth/", authHandler)
	}
	if apiHandler != nil {
		mux.Handle("/v1/", apiHandler)
	}
	return mux
}

type runtime struct {
	cfg          Config
	db           *sql.DB
	publisher    *relay.FranzPublisher
	consumer     *platform.Consumer
	server       *http.Server
	readiness    *readiness
	relayOwner   string
	workerCancel context.CancelFunc
	workerDone   *sync.WaitGroup
	closeOwned   func()
	closeOnce    sync.Once
}

func newRuntime(ctx context.Context, cfg Config) (*runtime, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	owner, err := newRelayOwner()
	if err != nil {
		return nil, fmt.Errorf("create relay owner: %w", err)
	}

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database failed; check %s", envDatabaseURL)
	}
	closeDB := true
	defer func() {
		if closeDB {
			_ = db.Close()
		}
	}()
	startupCtx, cancel := context.WithTimeout(ctx, defaultStartup)
	defer cancel()
	if err := db.PingContext(startupCtx); err != nil {
		return nil, fmt.Errorf("database ping failed; check %s", envDatabaseURL)
	}
	if err := erp.Migrate(startupCtx, db); err != nil {
		return nil, fmt.Errorf("ERP migration failed")
	}
	if err := platform.Migrate(startupCtx, db); err != nil {
		return nil, fmt.Errorf("platform migration failed")
	}
	if err := identity.Migrate(startupCtx, db); err != nil {
		return nil, fmt.Errorf("identity migration failed")
	}

	identityService, err := identity.New(startupCtx, identity.Config{
		Issuer:       cfg.OIDCIssuer,
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret,
		Audience:     cfg.OIDCAudience,
		RedirectURL:  cfg.OIDCRedirectURL,
		SessionTTL:   cfg.SessionTTL,
		CookieSecure: cfg.CookieSecure,
		CookieName:   cfg.CookieName,
	})
	if err != nil {
		return nil, fmt.Errorf("OIDC initialization failed; check %s", envOIDCIssuer)
	}

	hub := api.NewHub()
	apiServer := api.NewServer(db, hub, identityService, identity.NewPolicy(identity.NewDirectory(cfg.Assignments...)))
	publisher, err := relay.NewFranzPublisher(cfg.BrokerSeeds...)
	if err != nil {
		return nil, fmt.Errorf("create relay client failed; check %s", envBrokerSeeds)
	}
	consumer, err := platform.NewConsumer(db, cfg.BrokerSeeds...)
	if err != nil {
		publisher.Close()
		return nil, fmt.Errorf("create consumer client failed; check %s", envBrokerSeeds)
	}
	closeClients := true
	defer func() {
		if closeClients {
			consumer.Close()
			publisher.Close()
		}
	}()
	brokerCtx, brokerCancel := context.WithTimeout(ctx, cfg.CycleTimeout)
	defer brokerCancel()
	if err := publisher.Ping(brokerCtx); err != nil {
		return nil, fmt.Errorf("broker ping failed; check %s", envBrokerSeeds)
	}
	if err := consumer.Ping(brokerCtx); err != nil {
		return nil, fmt.Errorf("consumer broker ping failed; check %s", envBrokerSeeds)
	}

	state := newReadiness("database", "migrations", "broker", "relay", "consumer")
	state.set("database", true)
	state.set("migrations", true)
	state.set("broker", true)
	r := &runtime{
		cfg:        cfg,
		db:         db,
		publisher:  publisher,
		consumer:   consumer,
		readiness:  state,
		relayOwner: owner,
		server: &http.Server{
			Addr:              cfg.ListenAddr,
			Handler:           composeHandler(identityService.Handler(), apiServer.Handler(), state),
			ReadHeaderTimeout: 5 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}
	r.closeOwned = func() {
		platform.SetAppliedNotifier(nil)
		consumer.Close()
		publisher.Close()
		_ = db.Close()
	}
	platform.SetAppliedNotifier(hub)
	closeDB = false
	closeClients = false
	return r, nil
}

func newRelayOwner() (string, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "seshatops-relay-" + hex.EncodeToString(raw[:]), nil
}

func (r *runtime) Run(ctx context.Context) error {
	if r == nil || r.server == nil || r.db == nil || r.publisher == nil || r.consumer == nil {
		return errors.New("runtime is incomplete")
	}
	if err := ctx.Err(); err != nil {
		r.closeOnce.Do(func() {
			if r.closeOwned != nil {
				r.closeOwned()
			}
		})
		return err
	}
	listener, err := net.Listen("tcp", r.cfg.ListenAddr)
	if err != nil {
		r.closeOnce.Do(func() {
			if r.closeOwned != nil {
				r.closeOwned()
			}
		})
		return fmt.Errorf("listen failed; check %s", envListenAddr)
	}
	workerCtx, cancel := context.WithCancel(ctx)
	r.workerCancel = cancel
	var workers sync.WaitGroup
	workers.Add(2)
	r.workerDone = &workers
	go func() {
		defer workers.Done()
		r.runRelayWorker(workerCtx)
	}()
	go func() {
		defer workers.Done()
		r.runConsumerWorker(workerCtx)
	}()

	serveErr := make(chan error, 1)
	go func() { serveErr <- r.server.Serve(listener) }()
	select {
	case <-ctx.Done():
		return r.shutdown()
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return r.shutdown()
		}
		_ = r.shutdown()
		return err
	}
}

func (r *runtime) runRelayWorker(ctx context.Context) {
	runWorker(ctx, "relay", r.cfg.RelayInterval, r.cfg.RetryBase, r.cfg.RetryMax, func(ctx context.Context) error {
		cycleCtx, cancel := context.WithTimeout(ctx, r.cfg.CycleTimeout)
		defer cancel()
		_, err := relay.DrainOnce(cycleCtx, r.db, r.publisher, r.relayOwner, relay.DefaultLeaseTTL, 100)
		return err
	}, func(healthy bool) { r.readiness.set("relay", healthy) })
}

func (r *runtime) runConsumerWorker(ctx context.Context) {
	runWorker(ctx, "consumer", r.cfg.PollTimeout, r.cfg.RetryBase, r.cfg.RetryMax, func(ctx context.Context) error {
		cycleCtx, cancel := context.WithTimeout(ctx, r.cfg.PollTimeout)
		defer cancel()
		_, err := r.consumer.ConsumeOnce(cycleCtx)
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			// An idle healthy broker and an unavailable broker can both have
			// no records before the poll deadline. Probe the broker so
			// readiness does not stay green during an outage.
			probeCtx, probeCancel := context.WithTimeout(ctx, r.cfg.CycleTimeout)
			defer probeCancel()
			return r.consumer.Ping(probeCtx)
		}
		return err
	}, func(healthy bool) { r.readiness.set("consumer", healthy) })
}

func (r *runtime) shutdown() error {
	if r == nil {
		return nil
	}
	r.readiness.set("relay", false)
	r.readiness.set("consumer", false)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), r.cfg.Shutdown)
	defer cancel()
	if r.server != nil {
		if err := r.server.Shutdown(shutdownCtx); err != nil {
			// An active SSE connection can remain open indefinitely. Force-close
			// only after the bounded graceful window expires.
			_ = r.server.Close()
		}
	}
	if r.workerCancel != nil {
		r.workerCancel()
	}
	if r.workerDone != nil {
		done := make(chan struct{})
		go func() {
			r.workerDone.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-shutdownCtx.Done():
		}
	}
	r.closeOnce.Do(func() {
		if r.closeOwned != nil {
			r.closeOwned()
		}
	})
	return nil
}
