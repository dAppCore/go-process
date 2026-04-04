package process

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	coreerr "dappco.re/go/core/log"
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

// Find resolves the program's absolute path using exec.LookPath.
// Returns ErrProgramNotFound (wrapped) if the binary is not on PATH.
//
// Example:
//
//	if err := p.Find(); err != nil { return err }
func (p *Program) Find() error {
	if p.Name == "" {
		return coreerr.E("Program.Find", "program name is empty", nil)
	}
	path, err := exec.LookPath(p.Name)
	if err != nil {
		return coreerr.E("Program.Find", fmt.Sprintf("%q: not found in PATH", p.Name), ErrProgramNotFound)
	}
	p.Path = path
	return nil
}

// Run executes the program with args in the current working directory.
// Returns trimmed combined stdout+stderr output and any error.
//
// Example:
//
//	out, err := p.Run(ctx, "hello")
func (p *Program) Run(ctx context.Context, args ...string) (string, error) {
	return p.RunDir(ctx, "", args...)
}

// RunDir executes the program with args in dir.
// Returns trimmed combined stdout+stderr output and any error.
// If dir is empty, the process inherits the caller's working directory.
//
// Example:
//
//	out, err := p.RunDir(ctx, "/tmp", "pwd")
func (p *Program) RunDir(ctx context.Context, dir string, args ...string) (string, error) {
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

	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdout = &out
	cmd.Stderr = &out
	if dir != "" {
		cmd.Dir = dir
	}

	if err := cmd.Run(); err != nil {
		return strings.TrimSpace(out.String()), coreerr.E("Program.RunDir", fmt.Sprintf("%q: command failed", p.Name), err)
	}
	return strings.TrimSpace(out.String()), nil
}
