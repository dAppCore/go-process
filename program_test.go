package process_test

import (
	"context"
	"os"
	"testing"
	"time"

	"dappco.re/go/core"
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

func TestProgram_Find_Good(t *testing.T) {
	p := &process.Program{Name: "echo"}
	require.NoError(t, p.Find())
	assert.NotEmpty(t, p.Path)
}

func TestProgram_FindUnknown_Bad(t *testing.T) {
	p := &process.Program{Name: "no-such-binary-xyzzy-42"}
	err := p.Find()
	require.Error(t, err)
	assert.ErrorIs(t, err, process.ErrProgramNotFound)
}

func TestProgram_FindEmpty_Bad(t *testing.T) {
	p := &process.Program{}
	require.Error(t, p.Find())
}

func TestProgram_Run_Good(t *testing.T) {
	p := &process.Program{Name: "echo"}
	require.NoError(t, p.Find())

	out, err := p.Run(testCtx(t), "hello")
	require.NoError(t, err)
	assert.Equal(t, "hello", out)
}

func TestProgram_RunFallback_Good(t *testing.T) {
	// Path is empty; RunDir should fall back to Name for OS PATH resolution.
	p := &process.Program{Name: "echo"}

	out, err := p.Run(testCtx(t), "fallback")
	require.NoError(t, err)
	assert.Equal(t, "fallback", out)
}

func TestProgram_RunDir_Good(t *testing.T) {
	p := &process.Program{Name: "pwd"}
	require.NoError(t, p.Find())

	dir := t.TempDir()

	out, err := p.RunDir(testCtx(t), dir)
	require.NoError(t, err)
	dirInfo, err := os.Stat(dir)
	require.NoError(t, err)
	outInfo, err := os.Stat(core.Trim(out))
	require.NoError(t, err)
	assert.True(t, os.SameFile(dirInfo, outInfo))
}

func TestProgram_RunFailure_Bad(t *testing.T) {
	p := &process.Program{Name: "false"}
	require.NoError(t, p.Find())

	_, err := p.Run(testCtx(t))
	require.Error(t, err)
}
