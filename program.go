package process

import (
	"context"
	"unicode"

	core "dappco.re/go"
	coreerr "dappco.re/go/log"
)

// ErrProgramNotFound is returned when Find cannot locate the binary on PATH.
// Callers may use errors.Is to detect this condition.
var ErrProgramNotFound = coreerr.E("", "program: binary not found in PATH", nil)

// ErrProgramContextRequired is returned when Run or RunDir is called without a context.
var ErrProgramContextRequired = coreerr.E("", "program: command context is required", nil)

// ErrProgramNameRequired is returned when Run or RunDir is called without a program name.
var ErrProgramNameRequired = coreerr.E("", "program: program name is empty", nil)

// Program represents a named executable located on the system PATH.
//
// Example:
//
//	git := &process.Program{Name: "git"}
//	if err := git.Find(); err != nil { return err }
//	out, err := git.Run(ctx, "status")
type Program struct {
	// Name is the binary name (e.g. "go", "node", "git").
	Name string
	// Path is the absolute path resolved by Find.
	// Example: "/usr/bin/git"
	// If empty, Run and RunDir fall back to Name for OS PATH resolution.
	Path string
}

// Find resolves the program's absolute path using the OS search path.
// Returns ErrProgramNotFound (wrapped) if the binary is not on PATH.
//
// Example:
//
//	if err := p.Find(); err != nil { return err }
func (p *Program) Find() core.Result {
	target := p.Path
	if target == "" {
		target = p.Name
	}
	if target == "" {
		return core.Fail(coreerr.E("Program.Find", "program name is empty", nil))
	}
	path, err := lookPath(target)
	if err != nil {
		return core.Fail(coreerr.E("Program.Find", core.Sprintf("%q: not found in PATH", target), ErrProgramNotFound))
	}
	p.Path = path
	return core.Ok(nil)
}

// Run executes the program with args in the current working directory.
// Returns trimmed combined stdout+stderr output and any error.
//
// Example:
//
//	out, err := p.Run(ctx, "hello")
func (p *Program) Run(ctx context.Context, args ...string) (string, goError) {
	return p.RunDir(ctx, "", args...)
}

// RunDir executes the program with args in dir.
// Returns trimmed combined stdout+stderr output and any error.
// If dir is empty, the process inherits the caller's working directory.
//
// Example:
//
//	out, err := p.RunDir(ctx, "/tmp", "pwd")
func (p *Program) RunDir(ctx context.Context, dir string, args ...string) (string, goError) {
	if ctx == nil {
		return "", coreerr.E("Program.RunDir", "program: command context is required", ErrProgramContextRequired)
	}

	binary := p.Path
	if binary == "" {
		binary = p.Name
	}

	if binary == "" {
		return "", coreerr.E("Program.RunDir", "program name is empty", ErrProgramNameRequired)
	}

	out := core.NewBuffer()
	cmd := commandContext(ctx, binary, args...)
	cmd.Stdout = out
	cmd.Stderr = out
	if dir != "" {
		cmd.Dir = dir
	}

	if err := cmd.Run(); err != nil {
		return trimRightSpace(out.String()), coreerr.E("Program.RunDir", core.Sprintf("%q: command failed", binary), err)
	}
	return trimRightSpace(out.String()), nil
}

func trimRightSpace(s string) string {
	i := len(s)
	for i > 0 {
		r, size := utf8LastRuneInString(s[:i])
		if !unicode.IsSpace(r) {
			break
		}
		i -= size
	}
	return s[:i]
}

func utf8LastRuneInString(s string) (rune, int) {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] < 0x80 || s[i]&0xc0 == 0xc0 {
			r := []rune(s[i:])
			if len(r) == 0 {
				return rune(s[i]), 1
			}
			return r[0], len(s) - i
		}
	}
	return 0, 0
}
