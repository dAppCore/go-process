package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// DaemonOptions configures daemon mode execution.
type DaemonOptions struct {
	// PIDFile path for single-instance enforcement.
	// Leave empty to skip PID file management.
	PIDFile string

	// ShutdownTimeout is the maximum time to wait for graceful shutdown.
	// Default: 30 seconds.
	ShutdownTimeout time.Duration

	// HealthAddr is the address for health check endpoints.
	// Example: ":8080", "127.0.0.1:9000"
	// Leave empty to disable health checks.
	HealthAddr string

	// HealthChecks are additional health check functions.
	HealthChecks []HealthCheck

	// OnReload is called when SIGHUP is received.
	// Use for config reloading. Leave nil to ignore SIGHUP.
	OnReload func() error
}

// Daemon manages daemon lifecycle: PID file, health server, graceful shutdown.
type Daemon struct {
	opts    DaemonOptions
	pid     *PIDFile
	health  *HealthServer
	running bool
	mu      sync.Mutex
}

// NewDaemon creates a daemon runner with the given options.
func NewDaemon(opts DaemonOptions) *Daemon {
	if opts.ShutdownTimeout == 0 {
		opts.ShutdownTimeout = 30 * time.Second
	}

	d := &Daemon{opts: opts}

	if opts.PIDFile != "" {
		d.pid = NewPIDFile(opts.PIDFile)
	}

	if opts.HealthAddr != "" {
		d.health = NewHealthServer(opts.HealthAddr)
		for _, check := range opts.HealthChecks {
			d.health.AddCheck(check)
		}
	}

	return d
}

// Start initialises the daemon (PID file, health server).
func (d *Daemon) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		return errors.New("daemon already running")
	}

	if d.pid != nil {
		if err := d.pid.Acquire(); err != nil {
			return err
		}
	}

	if d.health != nil {
		if err := d.health.Start(); err != nil {
			if d.pid != nil {
				_ = d.pid.Release()
			}
			return err
		}
	}

	d.running = true
	return nil
}

// Run blocks until the context is cancelled.
func (d *Daemon) Run(ctx context.Context) error {
	d.mu.Lock()
	if !d.running {
		d.mu.Unlock()
		return errors.New("daemon not started - call Start() first")
	}
	d.mu.Unlock()

	<-ctx.Done()

	return d.Stop()
}

// Stop performs graceful shutdown.
func (d *Daemon) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return nil
	}

	var errs []error

	shutdownCtx, cancel := context.WithTimeout(context.Background(), d.opts.ShutdownTimeout)
	defer cancel()

	if d.health != nil {
		d.health.SetReady(false)
		if err := d.health.Stop(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("health server: %w", err))
		}
	}

	if d.pid != nil {
		if err := d.pid.Release(); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("pid file: %w", err))
		}
	}

	d.running = false

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}

// SetReady sets the daemon readiness status for health checks.
func (d *Daemon) SetReady(ready bool) {
	if d.health != nil {
		d.health.SetReady(ready)
	}
}

// HealthAddr returns the health server address, or empty if disabled.
func (d *Daemon) HealthAddr() string {
	if d.health != nil {
		return d.health.Addr()
	}
	return ""
}
