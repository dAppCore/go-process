package process

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	coreerr "forge.lthn.ai/core/go-log"
)

// ErrProgramNotFound is returned when Find cannot locate the binary on PATH.
// Callers may use errors.Is to detect this condition.
var ErrProgramNotFound = coreerr.E("", "program: binary not found in PATH", nil)

// Program represents a named executable located on the system PATH.
// Create one with a Name, call Find to resolve its path, then Run or RunDir.
type Program struct {
	// Name is the binary name (e.g. "go", "node", "git").
	Name string
	// Path is the absolute path resolved by Find.
	// If empty, Run and RunDir fall back to Name for OS PATH resolution.
	Path string
}

// Find resolves the program's absolute path using exec.LookPath.
// Returns ErrProgramNotFound (wrapped) if the binary is not on PATH.
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
func (p *Program) Run(ctx context.Context, args ...string) (string, error) {
	return p.RunDir(ctx, "", args...)
}

// RunDir executes the program with args in dir.
// Returns trimmed combined stdout+stderr output and any error.
// If dir is empty, the process inherits the caller's working directory.
func (p *Program) RunDir(ctx context.Context, dir string, args ...string) (string, error) {
	binary := p.Path
	if binary == "" {
		binary = p.Name
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
