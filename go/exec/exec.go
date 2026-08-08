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

	resolved := commandContext(c.ctx, c.name, c.args...)
	if !resolved.OK {
		return resolved
	}
	c.cmd = resolved.Value.(*core.Cmd)

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

// commandContext resolves name to a concrete executable and builds the handle
// for it.
//
// Resolution failure is returned, never swallowed. Handing the bare name to
// os/exec instead is not a harmless fallback on Windows: when Dir is set,
// Cmd.Start resolves a separator-free Path relative to Dir, so an unresolved
// "git" is reported as `exec: "C:\...\some work dir\git"` — a confusing error
// naming a path nobody asked for, in place of the honest "not found on PATH".
func commandContext(ctx context.Context, name string, arg ...string) core.Result {
	resolved := lookPath(name)
	if !resolved.OK {
		return resolved
	}
	return core.Ok(&core.Cmd{
		Path: resolved.Value.(string),
		Args: append([]string{name}, arg...),
	})
}

// defaultPathExt is the extension list Windows itself assumes when %PATHEXT%
// is unset.
const defaultPathExt = ".COM;.EXE;.BAT;.CMD"

// lookPath resolves file to a runnable path, searching PATH when file carries
// no directory component.
//
// On Windows a command is named without its extension — "git", not "git.exe" —
// so every candidate is also tried with each %PATHEXT% suffix. Without that,
// no Windows executable is ever found by its bare name.
func lookPath(file string) core.Result {
	return lookPathWith(file, executableExtensions())
}

// lookPathWith is lookPath with the extension list supplied rather than read
// from the environment. Taking it as an argument is what lets the Windows
// resolution rules be pinned on a POSIX runner — the tests drive it with a
// fixture PATH and a fake %PATHEXT%, so no Windows box is needed to prove the
// logic and the CI lane is left to prove only the wiring.
func lookPathWith(file string, extensions []string) core.Result {
	if file == "" {
		return core.Fail(core.E("lookPath", "executable file not found in PATH", nil))
	}
	if containsSeparator(file) {
		if path, ok := firstExecutable(file, extensions); ok {
			return core.Ok(path)
		}
		return core.Fail(core.E("lookPath", core.Sprintf("executable file %q not found", file), nil))
	}

	for _, dir := range core.Split(core.Getenv("PATH"), string(core.PathListSeparator)) {
		if dir == "" {
			dir = "."
		}
		if path, ok := firstExecutable(core.PathJoin(dir, file), extensions); ok {
			return core.Ok(path)
		}
	}
	return core.Fail(core.E("lookPath", core.Sprintf("executable file %q not found in PATH", file), nil))
}

// firstExecutable returns the first of base's candidate spellings that names a
// runnable file.
func firstExecutable(base string, extensions []string) (string, bool) {
	for _, candidate := range executableCandidates(base, extensions) {
		if isExecutableWith(candidate, extensions) {
			return candidate, true
		}
	}
	return "", false
}

// executableCandidates returns the spellings of base to try, in order. With no
// extensions in play — every POSIX case — base stands alone. On Windows a base
// that already ends in a listed extension also stands alone; anything else is
// tried once per extension, so "git" becomes "git.com", "git.exe" and so on.
func executableCandidates(base string, extensions []string) []string {
	if len(extensions) == 0 || hasExecutableExtension(base, extensions) {
		return []string{base}
	}
	candidates := make([]string, 0, len(extensions))
	for _, extension := range extensions {
		candidates = append(candidates, base+extension)
	}
	return candidates
}

// hasExecutableExtension reports whether base already ends in one of the
// listed extensions. Windows filenames are case-insensitive, so the comparison
// is too.
func hasExecutableExtension(base string, extensions []string) bool {
	lowered := core.Lower(base)
	for _, extension := range extensions {
		if core.HasSuffix(lowered, extension) {
			return true
		}
	}
	return false
}

// executableExtensions returns the %PATHEXT% list, or nil off Windows where a
// command name is used exactly as written.
func executableExtensions() []string {
	if string(core.PathSeparator) != `\` {
		return nil
	}
	return parsePathExt(core.Getenv("PATHEXT"))
}

// parsePathExt normalises a %PATHEXT% value into lower-cased, dot-prefixed
// extensions, dropping blanks and duplicates. An unset or unusable value falls
// back to the set Windows assumes, so a stripped environment still resolves
// the common executables.
func parsePathExt(value string) []string {
	extensions := make([]string, 0, 8)
	seen := make(map[string]bool, 8)
	for _, field := range core.Split(value, ";") {
		extension := core.Lower(core.Trim(field))
		if extension == "" || extension == "." {
			continue
		}
		if !core.HasPrefix(extension, ".") {
			extension = "." + extension
		}
		if seen[extension] {
			continue
		}
		seen[extension] = true
		extensions = append(extensions, extension)
	}
	if len(extensions) == 0 && value != defaultPathExt {
		return parsePathExt(defaultPathExt)
	}
	return extensions
}

// containsSeparator reports whether file carries a directory component under
// either convention. Windows accepts '/' as well as '\', so a name spelled
// "bin/tool" there is a path to check directly, not a name to hunt on PATH.
func containsSeparator(file string) bool {
	if core.Contains(file, "/") {
		return true
	}
	separator := string(core.PathSeparator)
	return separator != "/" && core.Contains(file, separator)
}

func isExecutable(path string) bool {
	return isExecutableWith(path, executableExtensions())
}

// isExecutableWith applies the platform's own rule for "this can be run".
//
// POSIX asks the mode bits. Windows has no execute bit — os.Stat synthesises
// 0666, or 0444 for a read-only file — so mode&0111 is never set there and a
// mode test rejects every file, git.exe included. Under a non-empty extension
// list the question becomes whether the suffix is one %PATHEXT% names, which
// is what Windows itself keys on.
func isExecutableWith(path string, extensions []string) bool {
	stat := core.Stat(path)
	if !stat.OK {
		return false
	}
	info, ok := stat.Value.(core.FsFileInfo)
	if !ok || info.IsDir() {
		return false
	}
	if len(extensions) > 0 {
		return hasExecutableExtension(path, extensions)
	}
	return info.Mode()&0111 != 0
}
