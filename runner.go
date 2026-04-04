package process

import (
	"context"
	"sync"
	"time"

	coreerr "dappco.re/go/core/log"
)

// Runner orchestrates multiple processes with dependencies.
type Runner struct {
	service *Service
}

// ErrRunnerNoService is returned when a runner was created without a service.
var ErrRunnerNoService = coreerr.E("", "runner service is nil", nil)

// ErrRunnerInvalidSpecName is returned when a RunSpec name is empty or duplicated.
var ErrRunnerInvalidSpecName = coreerr.E("", "runner spec names must be non-empty and unique", nil)

// ErrRunnerContextRequired is returned when a runner method is called without a context.
var ErrRunnerContextRequired = coreerr.E("", "runner context is required", nil)

// NewRunner creates a runner for the given service.
//
// Example:
//
//	runner := process.NewRunner(svc)
func NewRunner(svc *Service) *Runner {
	return &Runner{service: svc}
}

// RunSpec defines a process to run with optional dependencies.
//
// Example:
//
//	spec := process.RunSpec{Name: "test", Command: "go", Args: []string{"test", "./..."}}
type RunSpec struct {
	// Name is a friendly identifier (e.g., "lint", "test").
	Name string
	// Command is the executable to run.
	Command string
	// Args are the command arguments.
	Args []string
	// Dir is the working directory.
	Dir string
	// Env are additional environment variables.
	Env []string
	// After lists spec names that must complete successfully first.
	After []string
	// AllowFailure if true, continues pipeline even if this spec fails.
	AllowFailure bool
}

// RunResult captures the outcome of a single process.
type RunResult struct {
	Name     string
	Spec     RunSpec
	ExitCode int
	Duration time.Duration
	Output   string
	// Error only reports start-time or orchestration failures. A started process
	// that exits non-zero uses ExitCode to report failure and leaves Error nil.
	Error   error
	Skipped bool
}

// Passed returns true if the process succeeded.
//
// Example:
//
//	if result.Passed() { ... }
func (r RunResult) Passed() bool {
	return !r.Skipped && r.Error == nil && r.ExitCode == 0
}

// RunAllResult is the aggregate result of running multiple specs.
type RunAllResult struct {
	Results  []RunResult
	Duration time.Duration
	Passed   int
	Failed   int
	Skipped  int
}

// Success returns true when no spec failed.
//
// Example:
//
//	if result.Success() { ... }
func (r RunAllResult) Success() bool {
	return r.Failed == 0
}

// RunAll executes specs respecting dependencies, parallelising where possible.
//
// Example:
//
//	result, err := runner.RunAll(ctx, specs)
func (r *Runner) RunAll(ctx context.Context, specs []RunSpec) (*RunAllResult, error) {
	if err := r.ensureService(); err != nil {
		return nil, err
	}
	if err := ensureRunnerContext(ctx); err != nil {
		return nil, err
	}
	if err := validateSpecs(specs); err != nil {
		return nil, err
	}
	start := time.Now()

	// Build dependency graph
	specMap := make(map[string]RunSpec)
	indexMap := make(map[string]int, len(specs))
	for _, spec := range specs {
		specMap[spec.Name] = spec
		indexMap[spec.Name] = len(indexMap)
	}

	// Track completion
	completed := make(map[string]*RunResult)
	var completedMu sync.Mutex

	results := make([]RunResult, len(specs))

	// Process specs in waves
	remaining := make(map[string]RunSpec)
	for _, spec := range specs {
		remaining[spec.Name] = spec
	}

	for len(remaining) > 0 {
		if err := ctx.Err(); err != nil {
			for name := range remaining {
				results[indexMap[name]] = cancelledRunResult("Runner.RunAll", remaining[name], err)
			}
			break
		}

		// Find specs ready to run (all dependencies satisfied)
		ready := make([]RunSpec, 0)
		for _, spec := range remaining {
			if r.canRun(spec, completed) {
				ready = append(ready, spec)
			}
		}

		if len(ready) == 0 && len(remaining) > 0 {
			// Deadlock - circular dependency or missing specs.
			// Keep the output aligned with the input order.
			for name := range remaining {
				results[indexMap[name]] = RunResult{
					Name:    name,
					Spec:    remaining[name],
					Skipped: true,
					Error:   coreerr.E("Runner.RunAll", "circular dependency or missing dependency", nil),
				}
			}
			break
		}

		// Run ready specs in parallel
		var wg sync.WaitGroup
		for _, spec := range ready {
			wg.Add(1)
			go func(spec RunSpec) {
				defer wg.Done()

				// Check if dependencies failed
				completedMu.Lock()
				shouldSkip := false
				for _, dep := range spec.After {
					if result, ok := completed[dep]; ok {
						if !result.Passed() && !specMap[dep].AllowFailure {
							shouldSkip = true
							break
						}
					}
				}
				completedMu.Unlock()

				var result RunResult
				if shouldSkip {
					result = RunResult{
						Name:    spec.Name,
						Spec:    spec,
						Skipped: true,
						Error:   coreerr.E("Runner.RunAll", "skipped due to dependency failure", nil),
					}
				} else {
					result = r.runSpec(ctx, spec)
				}

				completedMu.Lock()
				completed[spec.Name] = &result
				completedMu.Unlock()

				results[indexMap[spec.Name]] = result
			}(spec)
		}
		wg.Wait()

		// Remove completed from remaining
		for _, spec := range ready {
			delete(remaining, spec.Name)
		}
	}

	// Build aggregate result
	aggResult := &RunAllResult{
		Results:  results,
		Duration: time.Since(start),
	}

	for _, res := range results {
		if res.Skipped {
			aggResult.Skipped++
		} else if res.Passed() {
			aggResult.Passed++
		} else {
			aggResult.Failed++
		}
	}

	return aggResult, nil
}

func (r *Runner) ensureService() error {
	if r == nil || r.service == nil {
		return ErrRunnerNoService
	}
	return nil
}

// canRun checks if all dependencies are completed.
func (r *Runner) canRun(spec RunSpec, completed map[string]*RunResult) bool {
	for _, dep := range spec.After {
		if _, ok := completed[dep]; !ok {
			return false
		}
	}
	return true
}

// runSpec executes a single spec.
func (r *Runner) runSpec(ctx context.Context, spec RunSpec) RunResult {
	start := time.Now()

	proc, err := r.service.StartWithOptions(ctx, RunOptions{
		Command: spec.Command,
		Args:    spec.Args,
		Dir:     spec.Dir,
		Env:     spec.Env,
	})
	if err != nil {
		return RunResult{
			Name:     spec.Name,
			Spec:     spec,
			Duration: time.Since(start),
			Error:    err,
		}
	}

	<-proc.Done()

	var runErr error
	switch proc.Status {
	case StatusKilled:
		runErr = coreerr.E("Runner.runSpec", "process was killed", nil)
	case StatusExited:
		// Non-zero exits are surfaced through ExitCode; Error remains nil so
		// callers can distinguish execution failure from orchestration failure.
	case StatusFailed:
		runErr = coreerr.E("Runner.runSpec", "process failed to start", nil)
	}

	return RunResult{
		Name:     spec.Name,
		Spec:     spec,
		ExitCode: proc.ExitCode,
		Duration: proc.Duration,
		Output:   proc.Output(),
		Error:    runErr,
	}
}

// RunSequential executes specs one after another, stopping on first failure.
//
// Example:
//
//	result, err := runner.RunSequential(ctx, specs)
func (r *Runner) RunSequential(ctx context.Context, specs []RunSpec) (*RunAllResult, error) {
	if err := r.ensureService(); err != nil {
		return nil, err
	}
	if err := ensureRunnerContext(ctx); err != nil {
		return nil, err
	}
	if err := validateSpecs(specs); err != nil {
		return nil, err
	}
	start := time.Now()
	results := make([]RunResult, 0, len(specs))

	for _, spec := range specs {
		if err := ctx.Err(); err != nil {
			results = append(results, cancelledRunResult("Runner.RunSequential", spec, err))
			continue
		}

		result := r.runSpec(ctx, spec)
		results = append(results, result)

		if !result.Passed() && !spec.AllowFailure {
			// Mark remaining as skipped
			for i := len(results); i < len(specs); i++ {
				results = append(results, skippedRunResult("Runner.RunSequential", specs[i], nil))
			}
			break
		}
	}

	aggResult := &RunAllResult{
		Results:  results,
		Duration: time.Since(start),
	}

	for _, res := range results {
		if res.Skipped {
			aggResult.Skipped++
		} else if res.Passed() {
			aggResult.Passed++
		} else {
			aggResult.Failed++
		}
	}

	return aggResult, nil
}

// RunParallel executes all specs concurrently, regardless of dependencies.
//
// Example:
//
//	result, err := runner.RunParallel(ctx, specs)
func (r *Runner) RunParallel(ctx context.Context, specs []RunSpec) (*RunAllResult, error) {
	if err := r.ensureService(); err != nil {
		return nil, err
	}
	if err := ensureRunnerContext(ctx); err != nil {
		return nil, err
	}
	if err := validateSpecs(specs); err != nil {
		return nil, err
	}
	start := time.Now()
	results := make([]RunResult, len(specs))

	var wg sync.WaitGroup
	for i, spec := range specs {
		wg.Add(1)
		go func(i int, spec RunSpec) {
			defer wg.Done()
			if err := ctx.Err(); err != nil {
				results[i] = cancelledRunResult("Runner.RunParallel", spec, err)
				return
			}
			results[i] = r.runSpec(ctx, spec)
		}(i, spec)
	}
	wg.Wait()

	aggResult := &RunAllResult{
		Results:  results,
		Duration: time.Since(start),
	}

	for _, res := range results {
		if res.Skipped {
			aggResult.Skipped++
		} else if res.Passed() {
			aggResult.Passed++
		} else {
			aggResult.Failed++
		}
	}

	return aggResult, nil
}

func validateSpecs(specs []RunSpec) error {
	seen := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		if spec.Name == "" {
			return coreerr.E("Runner.validateSpecs", "runner spec name is required", ErrRunnerInvalidSpecName)
		}
		if _, ok := seen[spec.Name]; ok {
			return coreerr.E("Runner.validateSpecs", "runner spec name is duplicated", ErrRunnerInvalidSpecName)
		}
		seen[spec.Name] = struct{}{}
	}
	return nil
}

func ensureRunnerContext(ctx context.Context) error {
	if ctx == nil {
		return coreerr.E("Runner.ensureRunnerContext", "runner context is required", ErrRunnerContextRequired)
	}
	return nil
}

func skippedRunResult(op string, spec RunSpec, err error) RunResult {
	result := RunResult{
		Name:    spec.Name,
		Spec:    spec,
		Skipped: true,
	}
	if err != nil {
		result.ExitCode = 1
		result.Error = coreerr.E(op, "skipped", err)
	}
	return result
}

func cancelledRunResult(op string, spec RunSpec, err error) RunResult {
	result := skippedRunResult(op, spec, err)
	if result.Error == nil {
		result.ExitCode = 1
		result.Error = coreerr.E(op, "context cancelled", err)
	}
	return result
}
