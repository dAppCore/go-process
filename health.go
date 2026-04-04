package process

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	coreerr "dappco.re/go/core/log"
)

// HealthCheck is a function that returns nil if healthy.
type HealthCheck func() error

// HealthServer provides HTTP /health and /ready endpoints for process monitoring.
type HealthServer struct {
	addr     string
	server   *http.Server
	listener net.Listener
	mu       sync.Mutex
	ready    bool
	checks   []HealthCheck
}

// NewHealthServer creates a health check server on the given address.
func NewHealthServer(addr string) *HealthServer {
	return &HealthServer{
		addr:  addr,
		ready: true,
	}
}

// AddCheck registers a health check function.
func (h *HealthServer) AddCheck(check HealthCheck) {
	h.mu.Lock()
	h.checks = append(h.checks, check)
	h.mu.Unlock()
}

// SetReady sets the readiness status.
func (h *HealthServer) SetReady(ready bool) {
	h.mu.Lock()
	h.ready = ready
	h.mu.Unlock()
}

// Start begins serving health check endpoints.
func (h *HealthServer) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		checks := h.checks
		h.mu.Unlock()

		for _, check := range checks {
			if err := check(); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = fmt.Fprintf(w, "unhealthy: %v\n", err)
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "ok")
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		ready := h.ready
		h.mu.Unlock()

		if !ready {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintln(w, "not ready")
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "ready")
	})

	listener, err := net.Listen("tcp", h.addr)
	if err != nil {
		return coreerr.E("HealthServer.Start", fmt.Sprintf("failed to listen on %s", h.addr), err)
	}

	h.listener = listener
	h.server = &http.Server{Handler: mux}

	go func() {
		_ = h.server.Serve(listener)
	}()

	return nil
}

// Stop gracefully shuts down the health server.
func (h *HealthServer) Stop(ctx context.Context) error {
	if h.server == nil {
		return nil
	}
	return h.server.Shutdown(ctx)
}

// Addr returns the actual address the server is listening on.
func (h *HealthServer) Addr() string {
	if h.listener != nil {
		return h.listener.Addr().String()
	}
	return h.addr
}

// WaitForHealth polls a health endpoint until it responds 200 or the timeout
// (in milliseconds) expires. Returns true if healthy, false on timeout.
func WaitForHealth(addr string, timeoutMs int) bool {
	ok, _ := ProbeHealth(addr, timeoutMs)
	return ok
}

// ProbeHealth polls a health endpoint until it responds 200 or the timeout
// (in milliseconds) expires. Returns the health status and the last observed
// failure reason if the endpoint never becomes healthy.
func ProbeHealth(addr string, timeoutMs int) (bool, string) {
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	url := fmt.Sprintf("http://%s/health", addr)

	client := &http.Client{Timeout: 2 * time.Second}
	var lastReason string

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true, ""
			}
			lastReason = strings.TrimSpace(string(body))
			if lastReason == "" {
				lastReason = resp.Status
			}
		} else {
			lastReason = err.Error()
		}
		time.Sleep(200 * time.Millisecond)
	}

	if lastReason == "" {
		lastReason = "health check timed out"
	}
	return false, lastReason
}
