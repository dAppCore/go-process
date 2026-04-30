package process

import (
	"context"
	// Note: AX-6 — internal concurrency primitive; structural per RFC §2
	"sync"
	"time"

	core "dappco.re/go"
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
	// PID, Health, Project, Binary, and Started are filled automatically.
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
func (d *Daemon) Start() core.Result {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		return core.Fail(core.E("Daemon.Start", "daemon already running", nil))
	}

	if d.pid != nil {
		if r := d.pid.Acquire(); !r.OK {
			return r
		}
	}

	if d.health != nil {
		if r := d.health.Start(); !r.OK {
			if d.pid != nil {
				if release := d.pid.Release(); !release.OK {
					return core.Fail(core.ErrorJoin(r.Value.(error), release.Value.(error)))
				}
			}
			return r
		}
	}

	// Auto-register if registry is set
	if d.opts.Registry != nil {
		entry := d.opts.RegistryEntry
		entry.PID = core.Getpid()
		if d.health != nil {
			entry.Health = d.health.Addr()
		}
		if entry.Project == "" {
			if wd := core.Getwd(); wd.OK {
				entry.Project = wd.Value.(string)
			}
		}
		if entry.Binary == "" {
			args := core.Args()
			if len(args) > 0 {
				entry.Binary = args[0]
			}
		}
		if r := d.opts.Registry.Register(entry); !r.OK {
			errs := []error{r.Value.(error)}
			if d.health != nil {
				if stop := d.health.Stop(context.Background()); !stop.OK {
					errs = append(errs, stop.Value.(error))
				}
			}
			if d.pid != nil {
				if release := d.pid.Release(); !release.OK {
					errs = append(errs, release.Value.(error))
				}
			}
			return core.Fail(core.E("Daemon.Start", "registry", core.ErrorJoin(errs...)))
		}
	}

	d.running = true
	return core.Ok(nil)
}

// Run blocks until the context is cancelled.
//
// Example:
//
//	if err := daemon.Run(ctx); err != nil { return err }
func (d *Daemon) Run(ctx context.Context) core.Result {
	if ctx == nil {
		return core.Fail(core.E("Daemon.Run", "daemon context is required", ErrDaemonContextRequired))
	}

	d.mu.Lock()
	if !d.running {
		d.mu.Unlock()
		return core.Fail(core.E("Daemon.Run", "daemon not started - call Start() first", nil))
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
func (d *Daemon) Stop() core.Result {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return core.Ok(nil)
	}

	var errs []error

	shutdownCtx, cancel := context.WithTimeout(context.Background(), d.opts.ShutdownTimeout)
	defer cancel()

	// Mark the daemon unavailable before tearing down listeners or registry state.
	if d.health != nil {
		d.health.SetReady(false)
	}

	if d.opts.Registry != nil {
		if r := d.opts.Registry.Unregister(d.opts.RegistryEntry.Code, d.opts.RegistryEntry.Daemon); !r.OK {
			errs = append(errs, core.E("Daemon.Stop", "registry", r.Value.(error)))
		}
	}

	if d.pid != nil {
		if r := d.pid.Release(); !r.OK && !core.IsNotExist(r.Value.(error)) {
			errs = append(errs, core.E("Daemon.Stop", "pid file", r.Value.(error)))
		}
	}

	if d.health != nil {
		if r := d.health.Stop(shutdownCtx); !r.OK {
			errs = append(errs, core.E("Daemon.Stop", "health server", r.Value.(error)))
		}
	}

	d.running = false

	if len(errs) > 0 {
		return core.Fail(core.ErrorJoin(errs...))
	}
	return core.Ok(nil)
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

// ErrDaemonContextRequired is returned when Run is called without a context.
var ErrDaemonContextRequired = core.E("", "daemon context is required", nil)
