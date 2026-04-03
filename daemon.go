package process

import (
	"context"
	"sync"
	"time"

	"dappco.re/go/core"
)

// DaemonOptions configures daemon mode execution.
//
//	opts := process.DaemonOptions{PIDFile: "/tmp/process.pid", HealthAddr: "127.0.0.1:0"}
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

	// Registry for tracking this daemon. Leave nil to skip registration.
	Registry *Registry

	// RegistryEntry provides the code and daemon name for registration.
	// PID, Health, and Started are filled automatically.
	RegistryEntry DaemonEntry
}

// Daemon manages daemon lifecycle: PID file, health server, graceful shutdown.
//
//	daemon := process.NewDaemon(process.DaemonOptions{HealthAddr: "127.0.0.1:0"})
type Daemon struct {
	opts    DaemonOptions
	pid     *PIDFile
	health  *HealthServer
	running bool
	mu      sync.Mutex
}

// NewDaemon creates a daemon runner with the given options.
//
//	daemon := process.NewDaemon(process.DaemonOptions{PIDFile: "/tmp/process.pid"})
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
		return core.E("daemon.start", "daemon already running", nil)
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

	// Auto-register if registry is set
	if d.opts.Registry != nil {
		entry := d.opts.RegistryEntry
		entry.PID = currentPID()
		if d.health != nil {
			entry.Health = d.health.Addr()
		}
		if err := d.opts.Registry.Register(entry); err != nil {
			if d.health != nil {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), d.opts.ShutdownTimeout)
				_ = d.health.Stop(shutdownCtx)
				cancel()
			}
			if d.pid != nil {
				_ = d.pid.Release()
			}
			d.running = false
			return core.E("daemon.start", "registry", err)
		}
	}

	return nil
}

// Run blocks until the context is cancelled.
func (d *Daemon) Run(ctx context.Context) error {
	d.mu.Lock()
	if !d.running {
		d.mu.Unlock()
		return core.E("daemon.run", "daemon not started - call Start() first", nil)
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
			errs = append(errs, core.E("daemon.stop", "health server", err))
		}
	}

	if d.pid != nil {
		if err := d.pid.Release(); err != nil && !isNotExist(err) {
			errs = append(errs, core.E("daemon.stop", "pid file", err))
		}
	}

	// Auto-unregister
	if d.opts.Registry != nil {
		if err := d.opts.Registry.Unregister(d.opts.RegistryEntry.Code, d.opts.RegistryEntry.Daemon); err != nil {
			errs = append(errs, core.E("daemon.stop", "registry", err))
		}
	}

	d.running = false

	if len(errs) > 0 {
		return core.ErrorJoin(errs...)
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
