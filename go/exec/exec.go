package command

import (
	"context"

	core "dappco.re/go"
	goio "io"
)

// ErrCommandContextRequired is returned when a command is created without a context.
var ErrCommandContextRequired = core.E("", "exec: command context is required", nil)

// Options configures command execution.
type Options struct {
	Dir    string
	Env    []string
	Stdin  goio.Reader
	Stdout goio.Writer
	Stderr goio.Writer
	// Background runs the command asynchronously and returns from Run immediately.
	Background bool
}

// Command wraps os/exec.Command with logging and context.
//
// Example:
//
//	cmd := exec.Command(ctx, "go", "test", "./...")
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
	cmd    *core.Cmd
	logger Logger
}

// WithDir sets the working directory.
//
// Example:
//
//	cmd.WithDir("/tmp")
func (c *Cmd) WithDir(dir string) *Cmd {
	c.opts.Dir = dir
	return c
}

// WithEnv sets the environment variables.
//
// Example:
//
//	cmd.WithEnv([]string{"CGO_ENABLED=0"})
func (c *Cmd) WithEnv(env []string) *Cmd {
	c.opts.Env = env
	return c
}

// WithStdin sets stdin.
//
// Example:
//
//	cmd.WithStdin(strings.NewReader("input"))
func (c *Cmd) WithStdin(r goio.Reader) *Cmd {
	c.opts.Stdin = r
	return c
}

// WithStdout sets stdout.
//
// Example:
//
//	cmd.WithStdout(os.Stdout)
func (c *Cmd) WithStdout(w goio.Writer) *Cmd {
	c.opts.Stdout = w
	return c
}

// WithStderr sets stderr.
//
// Example:
//
//	cmd.WithStderr(os.Stderr)
func (c *Cmd) WithStderr(w goio.Writer) *Cmd {
	c.opts.Stderr = w
	return c
}

// WithLogger sets a custom logger for this command.
// If not set, the package default logger is used.
func (c *Cmd) WithLogger(l Logger) *Cmd {
	c.logger = l
	return c
}

// WithBackground configures whether Run should wait for the command to finish.
func (c *Cmd) WithBackground(background bool) *Cmd {
	c.opts.Background = background
	return c
}

// Start launches the command.
//
// Example:
//
//	if err := cmd.Start(); err != nil { return err }
func (c *Cmd) Start() core.Result {
	if r := c.prepare(); !r.OK {
		return r
	}
	c.logDebug("executing command")

	if err := c.cmd.Start(); err != nil {
		wrapped := wrapError("Cmd.Start", err, c.name, c.args)
		c.logError("command failed", wrapped)
		return wrapped
	}
	c.watchContext()

	if c.opts.Background {
		go func(cmd *core.Cmd) {
			if err := cmd.Wait(); err != nil {
				c.logError("background command failed", wrapError("Cmd.Start", err, c.name, c.args))
			}
		}(c.cmd)
	}

	return core.Ok(nil)
}

// Run executes the command and waits for it to finish.
// It automatically logs the command execution at debug level.
//
// Example:
//
//	if err := cmd.Run(); err != nil { return err }
func (c *Cmd) Run() core.Result {
	if c.opts.Background {
		return c.Start()
	}

	if r := c.prepare(); !r.OK {
		return r
	}
	c.logDebug("executing command")

	if err := c.cmd.Start(); err != nil {
		wrapped := wrapError("Cmd.Run", err, c.name, c.args)
		c.logError("command failed", wrapped)
		return wrapped
	}
	c.watchContext()
	if err := c.cmd.Wait(); err != nil {
		wrapped := wrapError("Cmd.Run", err, c.name, c.args)
		c.logError("command failed", wrapped)
		return wrapped
	}
	return core.Ok(nil)
}

// Output runs the command and returns its standard output.
//
// Example:
//
//	result := cmd.Output()
func (c *Cmd) Output() core.Result {
	if c.opts.Background {
		return core.Fail(core.E("Cmd.Output", "background execution is incompatible with Output", nil))
	}

	if r := c.prepare(); !r.OK {
		return r
	}
	c.logDebug("executing command")

	out, err := c.cmd.Output()
	if err != nil {
		wrapped := wrapError("Cmd.Output", err, c.name, c.args)
		c.logError("command failed", wrapped)
		return wrapped
	}
	return core.Ok(out)
}

// CombinedOutput runs the command and returns its combined standard output and standard error.
//
// Example:
//
//	result := cmd.CombinedOutput()
func (c *Cmd) CombinedOutput() core.Result {
	if c.opts.Background {
		return core.Fail(core.E("Cmd.CombinedOutput", "background execution is incompatible with CombinedOutput", nil))
	}

	if r := c.prepare(); !r.OK {
		return r
	}
	c.logDebug("executing command")

	out, err := c.cmd.CombinedOutput()
	if err != nil {
		wrapped := wrapError("Cmd.CombinedOutput", err, c.name, c.args)
		c.logError("command failed", wrapped)
		return wrapped
	}
	return core.Ok(out)
}

func (c *Cmd) prepare() core.Result {
	if c.ctx == nil {
		return core.Fail(core.E("Cmd.prepare", "exec: command context is required", ErrCommandContextRequired))
	}

	c.cmd = commandContext(c.ctx, c.name, c.args...)

	c.cmd.Dir = c.opts.Dir
	if len(c.opts.Env) > 0 {
		c.cmd.Env = append(core.Environ(), c.opts.Env...)
	}

	c.cmd.Stdin = c.opts.Stdin
	c.cmd.Stdout = c.opts.Stdout
	c.cmd.Stderr = c.opts.Stderr
	return core.Ok(nil)
}

func (c *Cmd) watchContext() {
	if c.ctx == nil || c.cmd == nil {
		return
	}
	go func() {
		<-c.ctx.Done()
		if c.cmd.Process != nil {
			if err := c.cmd.Process.Kill(); err != nil {
				core.Print(core.Stderr(), "command context kill failed: %s", err)
			}
		}
	}()
}

// RunQuiet executes the command suppressing stdout unless there is an error.
// Useful for internal commands.
//
// Example:
//
//	err := exec.RunQuiet(ctx, "go", "vet", "./...")
func RunQuiet(ctx context.Context, name string, args ...string) core.Result {
	stderr := core.NewBuffer()
	cmd := Command(ctx, name, args...).WithStderr(stderr)
	if r := cmd.Run(); !r.OK {
		// Include stderr in error message
		return core.Fail(core.E("RunQuiet", core.Trim(stderr.String()), r.Value.(error)))
	}
	return core.Ok(nil)
}

func wrapError(caller string, err error, name string, args []string) core.Result {
	cmdStr := core.Join(" ", append([]string{name}, args...)...)
	if exitErr, ok := err.(interface{ ExitCode() int }); ok {
		return core.Fail(core.E(caller, core.Sprintf("command %q failed with exit code %d", cmdStr, exitErr.ExitCode()), err))
	}
	return core.Fail(core.E(caller, core.Sprintf("failed to execute %q", cmdStr), err))
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

func (c *Cmd) logError(msg string, failure core.Result) {
	c.getLogger().Error(msg, "cmd", c.name, "args", core.Join(" ", c.args...), "err", failure.Error())
}

func commandContext(ctx context.Context, name string, arg ...string) *core.Cmd {
	path := name
	if result := lookPath(name); result.OK {
		path = result.Value.(string)
	}

	cmd := &core.Cmd{
		Path: path,
		Args: append([]string{name}, arg...),
	}
	return cmd
}

func lookPath(file string) core.Result {
	if file == "" {
		return core.Fail(core.E("lookPath", "executable file not found in PATH", nil))
	}
	if core.Contains(file, string(core.PathSeparator)) {
		if isExecutable(file) {
			return core.Ok(file)
		}
		return core.Fail(core.E("lookPath", core.Sprintf("executable file %q not found", file), nil))
	}

	for _, dir := range core.Split(core.Getenv("PATH"), string(core.PathListSeparator)) {
		if dir == "" {
			dir = "."
		}
		path := core.PathJoin(dir, file)
		if isExecutable(path) {
			return core.Ok(path)
		}
	}
	return core.Fail(core.E("lookPath", core.Sprintf("executable file %q not found in PATH", file), nil))
}

func isExecutable(path string) bool {
	stat := core.Stat(path)
	if !stat.OK {
		return false
	}
	info, ok := stat.Value.(core.FsFileInfo)
	if !ok || info.IsDir() {
		return false
	}
	return info.Mode()&0111 != 0
}
