package process

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	coreerr "dappco.re/go/core/log"
)

// DaemonOptions configures daemon mode execution.
//
// Example:
//
//	opts := process.DaemonOptions{
//	    PIDFile: "/var/run/myapp.pid",
//	    HealthAddr: "127.0.0.1:0",
//	}
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
type Daemon struct {
	opts    DaemonOptions
	pid     *PIDFile
	health  *HealthServer
	running bool
	mu      sync.Mutex
}

// NewDaemon creates a daemon runner with the given options.
//
// Example:
//
//	daemon := process.NewDaemon(process.DaemonOptions{HealthAddr: "127.0.0.1:0"})
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
//
// Example:
//
//	if err := daemon.Start(); err != nil { return err }
func (d *Daemon) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		return coreerr.E("Daemon.Start", "daemon already running", nil)
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

	// Auto-register if registry is set
	if d.opts.Registry != nil {
		entry := d.opts.RegistryEntry
		entry.PID = os.Getpid()
		if d.health != nil {
			entry.Health = d.health.Addr()
		}
		if err := d.opts.Registry.Register(entry); err != nil {
			if d.health != nil {
				_ = d.health.Stop(context.Background())
			}
			if d.pid != nil {
				_ = d.pid.Release()
			}
			return coreerr.E("Daemon.Start", "registry", err)
		}
	}

	d.running = true
	return nil
}

// Run blocks until the context is cancelled.
//
// Example:
//
//	if err := daemon.Run(ctx); err != nil { return err }
func (d *Daemon) Run(ctx context.Context) error {
	d.mu.Lock()
	if !d.running {
		d.mu.Unlock()
		return coreerr.E("Daemon.Run", "daemon not started - call Start() first", nil)
	}
	d.mu.Unlock()

	<-ctx.Done()

	return d.Stop()
}

// Stop performs graceful shutdown.
//
// Example:
//
//	_ = daemon.Stop()
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
			errs = append(errs, coreerr.E("Daemon.Stop", "health server", err))
		}
	}

	if d.pid != nil {
		if err := d.pid.Release(); err != nil && !os.IsNotExist(err) {
			errs = append(errs, coreerr.E("Daemon.Stop", "pid file", err))
		}
	}

	// Auto-unregister
	if d.opts.Registry != nil {
		_ = d.opts.Registry.Unregister(d.opts.RegistryEntry.Code, d.opts.RegistryEntry.Daemon)
	}

	d.running = false

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// SetReady sets the daemon readiness status for `/ready`.
//
// Example:
//
//	daemon.SetReady(false)
func (d *Daemon) SetReady(ready bool) {
	if d.health != nil {
		d.health.SetReady(ready)
	}
}

// Ready reports whether the daemon is currently ready for traffic.
//
// Example:
//
//	if daemon.Ready() {
//	    // expose the service to callers
//	}
func (d *Daemon) Ready() bool {
	if d.health != nil {
		return d.health.Ready()
	}
	return false
}

// HealthAddr returns the health server address, or empty if disabled.
//
// Example:
//
//	addr := daemon.HealthAddr()
func (d *Daemon) HealthAddr() string {
	if d.health != nil {
		return d.health.Addr()
	}
	return ""
}
