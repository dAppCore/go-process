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

// HealthCheck is a function that returns nil when the service is healthy.
type HealthCheck func() error

// HealthServer provides HTTP `/health` and `/ready` endpoints for process monitoring.
type HealthServer struct {
	addr     string
	server   *http.Server
	listener net.Listener
	mu       sync.RWMutex
	ready    bool
	checks   []HealthCheck
}

// NewHealthServer creates a health check server on the given address.
//
// Example:
//
//	server := process.NewHealthServer("127.0.0.1:0")
func NewHealthServer(addr string) *HealthServer {
	return &HealthServer{
		addr:  addr,
		ready: true,
	}
}

// AddCheck registers a health check function.
//
// Example:
//
//	server.AddCheck(func() error { return nil })
func (h *HealthServer) AddCheck(check HealthCheck) {
	h.mu.Lock()
	h.checks = append(h.checks, check)
	h.mu.Unlock()
}

// SetReady sets the readiness status used by `/ready`.
//
// Example:
//
//	server.SetReady(false)
func (h *HealthServer) SetReady(ready bool) {
	h.mu.Lock()
	h.ready = ready
	h.mu.Unlock()
}

// Ready reports whether `/ready` currently returns HTTP 200.
//
// Example:
//
//	if server.Ready() {
//	    // publish the service
//	}
func (h *HealthServer) Ready() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.ready
}

// Start begins serving health check endpoints.
//
// Example:
//
//	if err := server.Start(); err != nil { return err }
func (h *HealthServer) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		checks := h.checksSnapshot()

		for _, check := range checks {
			if check == nil {
				continue
			}
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
		h.mu.RLock()
		ready := h.ready
		h.mu.RUnlock()

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

	server := &http.Server{Handler: mux}
	h.mu.Lock()
	h.listener = listener
	h.server = server
	h.mu.Unlock()

	go func() {
		_ = server.Serve(listener)
	}()

	return nil
}

// checksSnapshot returns a stable copy of the registered health checks.
func (h *HealthServer) checksSnapshot() []HealthCheck {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.checks) == 0 {
		return nil
	}

	checks := make([]HealthCheck, len(h.checks))
	copy(checks, h.checks)
	return checks
}

// Stop gracefully shuts down the health server.
//
// Example:
//
//	_ = server.Stop(context.Background())
func (h *HealthServer) Stop(ctx context.Context) error {
	h.mu.Lock()
	server := h.server
	h.server = nil
	h.listener = nil
	h.ready = false
	h.mu.Unlock()

	if server == nil {
		return nil
	}
	return server.Shutdown(ctx)
}

// Addr returns the actual address the server is listening on.
//
// Example:
//
//	addr := server.Addr()
func (h *HealthServer) Addr() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.listener != nil {
		return h.listener.Addr().String()
	}
	return h.addr
}

// WaitForHealth polls `/health` until it responds 200 or the timeout expires.
//
// Example:
//
//	if !process.WaitForHealth("127.0.0.1:8080", 5_000) {
//	    return errors.New("service did not become ready")
//	}
func WaitForHealth(addr string, timeoutMs int) bool {
	ok, _ := ProbeHealth(addr, timeoutMs)
	return ok
}

// ProbeHealth polls `/health` until it responds 200 or the timeout expires.
// It returns the health status and the last observed failure reason.
//
// Example:
//
//	ok, reason := process.ProbeHealth("127.0.0.1:8080", 5_000)
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

// WaitForReady polls `/ready` until it responds 200 or the timeout expires.
//
// Example:
//
//	if !process.WaitForReady("127.0.0.1:8080", 5_000) {
//	    return errors.New("service did not become ready")
//	}
func WaitForReady(addr string, timeoutMs int) bool {
	ok, _ := ProbeReady(addr, timeoutMs)
	return ok
}

// ProbeReady polls `/ready` until it responds 200 or the timeout expires.
// It returns the readiness status and the last observed failure reason.
//
// Example:
//
//	ok, reason := process.ProbeReady("127.0.0.1:8080", 5_000)
func ProbeReady(addr string, timeoutMs int) (bool, string) {
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	url := fmt.Sprintf("http://%s/ready", addr)

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
		lastReason = "readiness check timed out"
	}
	return false, lastReason
}
