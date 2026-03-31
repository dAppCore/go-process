package process

import (
	"bytes"
	"context"
	"strconv"

	"dappco.re/go/core"
)

// ErrProgramNotFound is returned when Find cannot locate the binary on PATH.
// Callers may use core.Is to detect this condition.
var ErrProgramNotFound = core.E("", "program: binary not found in PATH", nil)

// Program represents a named executable located on the system PATH.
// Create one with a Name, call Find to resolve its path, then Run or RunDir.
//
//	p := &process.Program{Name: "go"}
type Program struct {
	// Name is the binary name (e.g. "go", "node", "git").
	Name string
	// Path is the absolute path resolved by Find.
	// If empty, Run and RunDir fall back to Name for OS PATH resolution.
	Path string
}

// Find resolves the program's absolute path using exec.LookPath.
// Returns ErrProgramNotFound (wrapped) if the binary is not on PATH.
//
//	err := p.Find()
func (p *Program) Find() error {
	if p.Name == "" {
		return core.E("program.find", "program name is empty", nil)
	}
	path, err := execLookPath(p.Name)
	if err != nil {
		return core.E("program.find", core.Concat(strconv.Quote(p.Name), ": not found in PATH"), ErrProgramNotFound)
	}
	p.Path = path
	return nil
}

// Run executes the program with args in the current working directory.
// Returns trimmed combined stdout+stderr output and any error.
//
//	out, err := p.Run(ctx, "version")
func (p *Program) Run(ctx context.Context, args ...string) (string, error) {
	return p.RunDir(ctx, "", args...)
}

// RunDir executes the program with args in dir.
// Returns trimmed combined stdout+stderr output and any error.
// If dir is empty, the process inherits the caller's working directory.
//
//	out, err := p.RunDir(ctx, "/workspace", "test", "./...")
func (p *Program) RunDir(ctx context.Context, dir string, args ...string) (string, error) {
	binary := p.Path
	if binary == "" {
		binary = p.Name
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var out bytes.Buffer
	cmd := execCommandContext(ctx, binary, args...)
	cmd.Stdout = &out
	cmd.Stderr = &out
	if dir != "" {
		cmd.Dir = dir
	}

	if err := cmd.Run(); err != nil {
		return string(bytes.TrimSpace(out.Bytes())), core.E("program.run", core.Concat(strconv.Quote(p.Name), ": command failed"), err)
	}
	return string(bytes.TrimSpace(out.Bytes())), nil
}
