package process_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	require.NoError(t, p.Find())
	assert.NotEmpty(t, p.Path)
}

func TestProgram_Find_UnknownBinary(t *testing.T) {
	p := &process.Program{Name: "no-such-binary-xyzzy-42"}
	err := p.Find()
	require.Error(t, err)
	assert.ErrorIs(t, err, process.ErrProgramNotFound)
}

func TestProgram_Find_EmptyName(t *testing.T) {
	p := &process.Program{}
	require.Error(t, p.Find())
}

func TestProgram_Run_ReturnsOutput(t *testing.T) {
	p := &process.Program{Name: "echo"}
	require.NoError(t, p.Find())

	out, err := p.Run(testCtx(t), "hello")
	require.NoError(t, err)
	assert.Equal(t, "hello", out)
}

func TestProgram_Run_WithoutFind_FallsBackToName(t *testing.T) {
	// Path is empty; RunDir should fall back to Name for OS PATH resolution.
	p := &process.Program{Name: "echo"}

	out, err := p.Run(testCtx(t), "fallback")
	require.NoError(t, err)
	assert.Equal(t, "fallback", out)
}

func TestProgram_RunDir_UsesDirectory(t *testing.T) {
	p := &process.Program{Name: "pwd"}
	require.NoError(t, p.Find())

	dir := t.TempDir()

	out, err := p.RunDir(testCtx(t), dir)
	require.NoError(t, err)
	// Resolve symlinks on both sides for portability (macOS uses /private/ prefix).
	canonicalDir, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	canonicalOut, err := filepath.EvalSymlinks(out)
	require.NoError(t, err)
	assert.Equal(t, canonicalDir, canonicalOut)
}

func TestProgram_Run_FailingCommand(t *testing.T) {
	p := &process.Program{Name: "false"}
	require.NoError(t, p.Find())

	_, err := p.Run(testCtx(t))
	require.Error(t, err)
}

func TestProgram_Run_NilContextRejected(t *testing.T) {
	p := &process.Program{Name: "echo"}

	_, err := p.Run(nil, "test")
	require.Error(t, err)
	assert.ErrorIs(t, err, process.ErrProgramContextRequired)
}

func TestProgram_RunDir_EmptyNameRejected(t *testing.T) {
	p := &process.Program{}

	_, err := p.RunDir(testCtx(t), "", "test")
	require.Error(t, err)
	assert.ErrorIs(t, err, process.ErrProgramNameRequired)
}
