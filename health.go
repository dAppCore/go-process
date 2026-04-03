package process

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"dappco.re/go/core"
)

// HealthCheck is a function that returns nil if healthy.
//
//	check := process.HealthCheck(func() error { return nil })
type HealthCheck func() error

// HealthServer provides HTTP /health and /ready endpoints for process monitoring.
//
//	hs := process.NewHealthServer("127.0.0.1:0")
type HealthServer struct {
	addr     string
	server   *http.Server
	listener net.Listener
	mu       sync.Mutex
	ready    bool
	checks   []HealthCheck
}

// NewHealthServer creates a health check server on the given address.
//
//	hs := process.NewHealthServer("127.0.0.1:0")
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
				_, _ = w.Write([]byte("unhealthy: " + err.Error() + "\n"))
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		ready := h.ready
		h.mu.Unlock()

		if !ready {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("not ready\n"))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready\n"))
	})

	listener, err := net.Listen("tcp", h.addr)
	if err != nil {
		return core.E("health.start", core.Concat("failed to listen on ", h.addr), err)
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
	h.mu.Lock()
	server := h.server
	h.server = nil
	h.listener = nil
	h.mu.Unlock()

	if server == nil {
		return nil
	}

	return server.Shutdown(ctx)
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
//
//	ok := process.WaitForHealth("127.0.0.1:9000", 2_000)
func WaitForHealth(addr string, timeoutMs int) bool {
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	url := core.Concat("http://", addr, "/health")

	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
		time.Sleep(200 * time.Millisecond)
	}

	return false
}
