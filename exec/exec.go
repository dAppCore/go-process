package exec

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"

	"dappco.re/go/core"
)

// Options configures command execution.
//
//	opts := exec.Options{Dir: "/workspace", Env: []string{"CI=1"}}
type Options struct {
	Dir    string
	Env    []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Command wraps `os/exec.Command` with logging and context.
//
//	cmd := exec.Command(ctx, "git", "status").WithDir("/workspace")
func Command(ctx context.Context, name string, args ...string) *Cmd {
	return &Cmd{
		name: name,
		args: args,
		ctx:  ctx,
	}
}

// Cmd represents a wrapped command.
type Cmd struct {
	name   string
	args   []string
	ctx    context.Context
	opts   Options
	cmd    *exec.Cmd
	logger Logger
}

// WithDir sets the working directory.
func (c *Cmd) WithDir(dir string) *Cmd {
	c.opts.Dir = dir
	return c
}

// WithEnv sets the environment variables.
func (c *Cmd) WithEnv(env []string) *Cmd {
	c.opts.Env = env
	return c
}

// WithStdin sets stdin.
func (c *Cmd) WithStdin(r io.Reader) *Cmd {
	c.opts.Stdin = r
	return c
}

// WithStdout sets stdout.
func (c *Cmd) WithStdout(w io.Writer) *Cmd {
	c.opts.Stdout = w
	return c
}

// WithStderr sets stderr.
func (c *Cmd) WithStderr(w io.Writer) *Cmd {
	c.opts.Stderr = w
	return c
}

// WithLogger sets a custom logger for this command.
// If not set, the package default logger is used.
func (c *Cmd) WithLogger(l Logger) *Cmd {
	c.logger = l
	return c
}

// Run executes the command and waits for it to finish.
// It automatically logs the command execution at debug level.
func (c *Cmd) Run() error {
	c.prepare()
	c.logDebug("executing command")

	if err := c.cmd.Run(); err != nil {
		wrapped := wrapError("Cmd.Run", err, c.name, c.args)
		c.logError("command failed", wrapped)
		return wrapped
	}
	return nil
}

// Output runs the command and returns its standard output.
func (c *Cmd) Output() ([]byte, error) {
	c.prepare()
	c.logDebug("executing command")

	out, err := c.cmd.Output()
	if err != nil {
		wrapped := wrapError("Cmd.Output", err, c.name, c.args)
		c.logError("command failed", wrapped)
		return nil, wrapped
	}
	return out, nil
}

// CombinedOutput runs the command and returns its combined standard output and standard error.
func (c *Cmd) CombinedOutput() ([]byte, error) {
	c.prepare()
	c.logDebug("executing command")

	out, err := c.cmd.CombinedOutput()
	if err != nil {
		wrapped := wrapError("Cmd.CombinedOutput", err, c.name, c.args)
		c.logError("command failed", wrapped)
		return out, wrapped
	}
	return out, nil
}

func (c *Cmd) prepare() {
	ctx := c.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	c.cmd = exec.CommandContext(ctx, c.name, c.args...)

	c.cmd.Dir = c.opts.Dir
	if len(c.opts.Env) > 0 {
		c.cmd.Env = append(os.Environ(), c.opts.Env...)
	}

	c.cmd.Stdin = c.opts.Stdin
	c.cmd.Stdout = c.opts.Stdout
	c.cmd.Stderr = c.opts.Stderr
}

// RunQuiet executes the command suppressing stdout unless there is an error.
// Useful for internal commands.
//
//	_ = exec.RunQuiet(ctx, "go", "test", "./...")
func RunQuiet(ctx context.Context, name string, args ...string) error {
	var stderr bytes.Buffer
	cmd := Command(ctx, name, args...).WithStderr(&stderr)
	if err := cmd.Run(); err != nil {
		return core.E("RunQuiet", core.Trim(stderr.String()), err)
	}
	return nil
}

func wrapError(caller string, err error, name string, args []string) error {
	cmdStr := commandString(name, args)
	if exitErr, ok := err.(*exec.ExitError); ok {
		return core.E(caller, core.Sprintf("command %q failed with exit code %d", cmdStr, exitErr.ExitCode()), err)
	}
	return core.E(caller, core.Sprintf("failed to execute %q", cmdStr), err)
}

func (c *Cmd) getLogger() Logger {
	if c.logger != nil {
		return c.logger
	}
	return defaultLogger
}

func (c *Cmd) logDebug(msg string) {
	c.getLogger().Debug(msg, "cmd", c.name, "args", core.Join(" ", c.args...))
}

func (c *Cmd) logError(msg string, err error) {
	c.getLogger().Error(msg, "cmd", c.name, "args", core.Join(" ", c.args...), "err", err)
}

func commandString(name string, args []string) string {
	if len(args) == 0 {
		return name
	}
	parts := append([]string{name}, args...)
	return core.Join(" ", parts...)
}
