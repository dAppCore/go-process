package process_test

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	process "dappco.re/go/core/process"
)

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestProgram_Find_KnownBinary(t *testing.T) {
	p := &process.Program{Name: "echo"}
	if err := p.Find(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Path == "" {
		t.Fatal("expected non-empty path")
	}
}

func TestProgram_Find_UnknownBinary(t *testing.T) {
	p := &process.Program{Name: "no-such-binary-xyzzy-42"}
	err := p.Find()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, process.ErrProgramNotFound) {
		t.Fatalf("expected ErrProgramNotFound, got %v", err)
	}
}

func TestProgram_Find_UsesExistingPath(t *testing.T) {
	path, err := exec.LookPath("echo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p := &process.Program{Path: path}
	if err := p.Find(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Path != path {
		t.Fatalf("want %v, got %v", path, p.Path)
	}
}

func TestProgram_Find_PrefersExistingPathOverName(t *testing.T) {
	path, err := exec.LookPath("echo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p := &process.Program{
		Name: "no-such-binary-xyzzy-42",
		Path: path,
	}

	if err := p.Find(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Path != path {
		t.Fatalf("want %v, got %v", path, p.Path)
	}
}

func TestProgram_Find_EmptyName(t *testing.T) {
	p := &process.Program{}
	if err := p.Find(); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestProgram_Run_ReturnsOutput(t *testing.T) {
	p := &process.Program{Name: "echo"}
	if err := p.Find(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out, err := p.Run(testCtx(t), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "hello" {
		t.Fatalf("want %q, got %q", "hello", out)
	}
}

func TestProgram_Run_PreservesLeadingWhitespace(t *testing.T) {
	p := &process.Program{Name: "sh"}
	if err := p.Find(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out, err := p.Run(testCtx(t), "-c", "printf '  hello  \n'")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "  hello" {
		t.Fatalf("want %q, got %q", "  hello", out)
	}
}

func TestProgram_Run_WithoutFind_FallsBackToName(t *testing.T) {
	// Path is empty; RunDir should fall back to Name for OS PATH resolution.
	p := &process.Program{Name: "echo"}

	out, err := p.Run(testCtx(t), "fallback")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "fallback" {
		t.Fatalf("want %q, got %q", "fallback", out)
	}
}

func TestProgram_RunDir_UsesDirectory(t *testing.T) {
	p := &process.Program{Name: "pwd"}
	if err := p.Find(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dir := t.TempDir()

	out, err := p.RunDir(testCtx(t), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Resolve symlinks on both sides for portability (macOS uses /private/ prefix).
	canonicalDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	canonicalOut, err := filepath.EvalSymlinks(out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if canonicalDir != canonicalOut {
		t.Fatalf("want %q, got %q", canonicalDir, canonicalOut)
	}
}

func TestProgram_Run_FailingCommand(t *testing.T) {
	p := &process.Program{Name: "false"}
	if err := p.Find(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err := p.Run(testCtx(t))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestProgram_Run_NilContextRejected(t *testing.T) {
	p := &process.Program{Name: "echo"}

	_, err := p.Run(nil, "test")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, process.ErrProgramContextRequired) {
		t.Fatalf("expected ErrProgramContextRequired, got %v", err)
	}
}

func TestProgram_RunDir_EmptyNameRejected(t *testing.T) {
	p := &process.Program{}

	_, err := p.RunDir(testCtx(t), "", "test")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, process.ErrProgramNameRequired) {
		t.Fatalf("expected ErrProgramNameRequired, got %v", err)
	}
}
