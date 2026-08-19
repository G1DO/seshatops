package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestComposedHandlerRoutesHealthAuthAndAPI(t *testing.T) {
	state := newReadiness("database")
	auth := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Route", "auth")
		w.WriteHeader(http.StatusAccepted)
	})
	apiHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Route", "api")
		w.WriteHeader(http.StatusAccepted)
	})
	ts := httptest.NewServer(composeHandler(auth, apiHandler, state))
	t.Cleanup(ts.Close)

	resp := get(t, ts.URL+"/livez")
	if resp.StatusCode != http.StatusOK || readBody(t, resp) != `{"status":"alive"}
` {
		t.Fatalf("live response = %d", resp.StatusCode)
	}
	resp = get(t, ts.URL+"/readyz")
	if resp.StatusCode != http.StatusServiceUnavailable || readBody(t, resp) != `{"status":"not_ready"}
` {
		t.Fatalf("initial ready response = %d", resp.StatusCode)
	}

	resp = get(t, ts.URL+"/auth/login")
	if resp.StatusCode != http.StatusAccepted || resp.Header.Get("X-Route") != "auth" {
		t.Fatalf("auth response = %d route=%q", resp.StatusCode, resp.Header.Get("X-Route"))
	}
	_ = resp.Body.Close()
	resp = get(t, ts.URL+"/v1/tenants/example")
	if resp.StatusCode != http.StatusAccepted || resp.Header.Get("X-Route") != "api" {
		t.Fatalf("api response = %d route=%q", resp.StatusCode, resp.Header.Get("X-Route"))
	}
	_ = resp.Body.Close()

	state.set("database", true)
	resp = get(t, ts.URL+"/readyz")
	if resp.StatusCode != http.StatusOK || readBody(t, resp) != `{"status":"ready"}
` {
		t.Fatalf("ready response = %d", resp.StatusCode)
	}
}

func TestReadinessTransitionsBackToUnavailable(t *testing.T) {
	state := newReadiness("database", "broker")
	state.set("database", true)
	state.set("broker", true)
	if !state.ready() {
		t.Fatal("expected ready state")
	}
	state.set("broker", false)
	if state.ready() {
		t.Fatal("expected unavailable state after broker failure")
	}
}

func TestRuntimeShutdownStopsHTTPAndClosesOwnedResources(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	addr := listener.Addr().String()
	deadline := time.Now().Add(time.Second)
	for {
		resp, getErr := http.Get("http://" + addr)
		if getErr == nil {
			_ = resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not start: %v", getErr)
		}
		time.Sleep(time.Millisecond)
	}

	workerCtx, cancelWorker := context.WithCancel(context.Background())
	var workers sync.WaitGroup
	workers.Add(1)
	go func() {
		defer workers.Done()
		<-workerCtx.Done()
	}()
	closed := 0
	r := &runtime{
		cfg:          Config{Shutdown: time.Second},
		server:       server,
		readiness:    newReadiness("relay", "consumer"),
		workerCancel: cancelWorker,
		workerDone:   &workers,
		closeOwned:   func() { closed++ },
	}

	if err := r.shutdown(); err != nil {
		t.Fatal(err)
	}
	if closed != 1 {
		t.Fatalf("close count = %d", closed)
	}
	if err := r.shutdown(); err != nil {
		t.Fatal(err)
	}
	if closed != 1 {
		t.Fatalf("close count after repeat = %d", closed)
	}
	if err := <-serveDone; err != http.ErrServerClosed {
		t.Fatalf("serve error = %v", err)
	}

	client := &http.Client{}
	resp, err := client.Get("http://" + addr + "/livez")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("HTTP remained available after shutdown")
	}
}

func get(t *testing.T, rawURL string) *http.Response {
	t.Helper()
	resp, err := http.Get(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
