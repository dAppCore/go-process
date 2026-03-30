# exec
**Import:** `dappco.re/go/core/process/exec`
**Files:** 3

## Types

### `Options`
`struct`

Command execution options used by `Cmd`.

Fields:
- `Dir string`: Working directory.
- `Env []string`: Environment entries appended to `os.Environ()` when non-empty.
- `Stdin io.Reader`: Reader assigned to command stdin.
- `Stdout io.Writer`: Writer assigned to command stdout.
- `Stderr io.Writer`: Writer assigned to command stderr.

### `Cmd`
`struct`

Wrapped command with chainable configuration methods.

Exported fields:
- None.

### `Logger`
`interface`

Command-execution logger.

Methods:
- `Debug(msg string, keyvals ...any)`: Logs a debug-level message.
- `Error(msg string, keyvals ...any)`: Logs an error-level message.

### `NopLogger`
`struct`

No-op `Logger` implementation.

Exported fields:
- None.

## Functions

### Package Functions

- `func Command(ctx context.Context, name string, args ...string) *Cmd`: Returns a `Cmd` for the supplied context, executable name, and arguments.
- `func RunQuiet(ctx context.Context, name string, args ...string) error`: Runs a command with stderr captured into a buffer and returns `core.E("RunQuiet", core.Trim(stderr.String()), err)` on failure.
- `func SetDefaultLogger(l Logger)`: Sets the package-level default logger. Passing `nil` replaces it with `NopLogger`.
- `func DefaultLogger() Logger`: Returns the package-level default logger.

### `Cmd` Methods

- `func (c *Cmd) WithDir(dir string) *Cmd`: Sets `Options.Dir` and returns the same command.
- `func (c *Cmd) WithEnv(env []string) *Cmd`: Sets `Options.Env` and returns the same command.
- `func (c *Cmd) WithStdin(r io.Reader) *Cmd`: Sets `Options.Stdin` and returns the same command.
- `func (c *Cmd) WithStdout(w io.Writer) *Cmd`: Sets `Options.Stdout` and returns the same command.
- `func (c *Cmd) WithStderr(w io.Writer) *Cmd`: Sets `Options.Stderr` and returns the same command.
- `func (c *Cmd) WithLogger(l Logger) *Cmd`: Sets a command-specific logger and returns the same command.
- `func (c *Cmd) Run() error`: Prepares the underlying `exec.Cmd`, logs `"executing command"`, runs it, and wraps failures with `wrapError("Cmd.Run", ...)`.
- `func (c *Cmd) Output() ([]byte, error)`: Prepares the underlying `exec.Cmd`, logs `"executing command"`, returns stdout bytes, and wraps failures with `wrapError("Cmd.Output", ...)`.
- `func (c *Cmd) CombinedOutput() ([]byte, error)`: Prepares the underlying `exec.Cmd`, logs `"executing command"`, returns combined stdout and stderr, and wraps failures with `wrapError("Cmd.CombinedOutput", ...)`.

### `NopLogger` Methods

- `func (NopLogger) Debug(string, ...any)`: Discards the message.
- `func (NopLogger) Error(string, ...any)`: Discards the message.
