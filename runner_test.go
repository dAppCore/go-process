package process

import (
	"context"
	"testing"

	framework "dappco.re/go/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRunner(t *testing.T) *Runner {
	t.Helper()

	c := framework.New()
	r := Register(c)
	require.True(t, r.OK)
	return NewRunner(r.Value.(*Service))
}

func TestRunner_RunSequential_Good(t *testing.T) {
	t.Run("all pass", func(t *testing.T) {
		runner := newTestRunner(t)

		result, err := runner.RunSequential(context.Background(), []RunSpec{
			{Name: "first", Command: "echo", Args: []string{"1"}},
			{Name: "second", Command: "echo", Args: []string{"2"}},
			{Name: "third", Command: "echo", Args: []string{"3"}},
		})
		require.NoError(t, err)

		assert.True(t, result.Success())
		assert.Equal(t, 3, result.Passed)
		assert.Equal(t, 0, result.Failed)
		assert.Equal(t, 0, result.Skipped)
	})

	t.Run("stops on failure", func(t *testing.T) {
		runner := newTestRunner(t)

		result, err := runner.RunSequential(context.Background(), []RunSpec{
			{Name: "first", Command: "echo", Args: []string{"1"}},
			{Name: "fails", Command: "sh", Args: []string{"-c", "exit 1"}},
			{Name: "third", Command: "echo", Args: []string{"3"}},
		})
		require.NoError(t, err)

		assert.False(t, result.Success())
		assert.Equal(t, 1, result.Passed)
		assert.Equal(t, 1, result.Failed)
		assert.Equal(t, 1, result.Skipped)
	})

	t.Run("allow failure continues", func(t *testing.T) {
		runner := newTestRunner(t)

		result, err := runner.RunSequential(context.Background(), []RunSpec{
			{Name: "first", Command: "echo", Args: []string{"1"}},
			{Name: "fails", Command: "sh", Args: []string{"-c", "exit 1"}, AllowFailure: true},
			{Name: "third", Command: "echo", Args: []string{"3"}},
		})
		require.NoError(t, err)

		// Still counts as failed but pipeline continues
		assert.Equal(t, 2, result.Passed)
		assert.Equal(t, 1, result.Failed)
		assert.Equal(t, 0, result.Skipped)
	})
}

func TestRunner_RunParallel_Good(t *testing.T) {
	t.Run("all run concurrently", func(t *testing.T) {
		runner := newTestRunner(t)

		result, err := runner.RunParallel(context.Background(), []RunSpec{
			{Name: "first", Command: "echo", Args: []string{"1"}},
			{Name: "second", Command: "echo", Args: []string{"2"}},
			{Name: "third", Command: "echo", Args: []string{"3"}},
		})
		require.NoError(t, err)

		assert.True(t, result.Success())
		assert.Equal(t, 3, result.Passed)
		assert.Len(t, result.Results, 3)
	})

	t.Run("failure doesnt stop others", func(t *testing.T) {
		runner := newTestRunner(t)

		result, err := runner.RunParallel(context.Background(), []RunSpec{
			{Name: "first", Command: "echo", Args: []string{"1"}},
			{Name: "fails", Command: "sh", Args: []string{"-c", "exit 1"}},
			{Name: "third", Command: "echo", Args: []string{"3"}},
		})
		require.NoError(t, err)

		assert.False(t, result.Success())
		assert.Equal(t, 2, result.Passed)
		assert.Equal(t, 1, result.Failed)
	})
}

func TestRunner_RunAll_Good(t *testing.T) {
	t.Run("respects dependencies", func(t *testing.T) {
		runner := newTestRunner(t)

		result, err := runner.RunAll(context.Background(), []RunSpec{
			{Name: "third", Command: "echo", Args: []string{"3"}, After: []string{"second"}},
			{Name: "first", Command: "echo", Args: []string{"1"}},
			{Name: "second", Command: "echo", Args: []string{"2"}, After: []string{"first"}},
		})
		require.NoError(t, err)

		assert.True(t, result.Success())
		assert.Equal(t, 3, result.Passed)
	})

	t.Run("skips dependents on failure", func(t *testing.T) {
		runner := newTestRunner(t)

		result, err := runner.RunAll(context.Background(), []RunSpec{
			{Name: "first", Command: "sh", Args: []string{"-c", "exit 1"}},
			{Name: "second", Command: "echo", Args: []string{"2"}, After: []string{"first"}},
			{Name: "third", Command: "echo", Args: []string{"3"}, After: []string{"second"}},
		})
		require.NoError(t, err)

		assert.False(t, result.Success())
		assert.Equal(t, 0, result.Passed)
		assert.Equal(t, 1, result.Failed)
		assert.Equal(t, 2, result.Skipped)
	})

	t.Run("parallel independent specs", func(t *testing.T) {
		runner := newTestRunner(t)

		// These should run in parallel since they have no dependencies
		result, err := runner.RunAll(context.Background(), []RunSpec{
			{Name: "a", Command: "echo", Args: []string{"a"}},
			{Name: "b", Command: "echo", Args: []string{"b"}},
			{Name: "c", Command: "echo", Args: []string{"c"}},
			{Name: "final", Command: "echo", Args: []string{"done"}, After: []string{"a", "b", "c"}},
		})
		require.NoError(t, err)

		assert.True(t, result.Success())
		assert.Equal(t, 4, result.Passed)
	})

	t.Run("preserves input order", func(t *testing.T) {
		runner := newTestRunner(t)

		specs := []RunSpec{
			{Name: "third", Command: "echo", Args: []string{"3"}, After: []string{"second"}},
			{Name: "first", Command: "echo", Args: []string{"1"}},
			{Name: "second", Command: "echo", Args: []string{"2"}, After: []string{"first"}},
		}

		result, err := runner.RunAll(context.Background(), specs)
		require.NoError(t, err)

		require.Len(t, result.Results, len(specs))
		for i, res := range result.Results {
			assert.Equal(t, specs[i].Name, res.Name)
		}
	})
}

func TestRunner_CircularDeps_Bad(t *testing.T) {
	t.Run("circular dependency counts as failed", func(t *testing.T) {
		runner := newTestRunner(t)

		result, err := runner.RunAll(context.Background(), []RunSpec{
			{Name: "a", Command: "echo", Args: []string{"a"}, After: []string{"b"}},
			{Name: "b", Command: "echo", Args: []string{"b"}, After: []string{"a"}},
		})
		require.NoError(t, err)

		assert.False(t, result.Success())
		assert.Equal(t, 2, result.Failed)
		assert.Equal(t, 0, result.Skipped)
	})
}

func TestRunResult_Passed_Good(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		r := RunResult{ExitCode: 0}
		assert.True(t, r.Passed())
	})

	t.Run("non-zero exit", func(t *testing.T) {
		r := RunResult{ExitCode: 1}
		assert.False(t, r.Passed())
	})

	t.Run("skipped", func(t *testing.T) {
		r := RunResult{ExitCode: 0, Skipped: true}
		assert.False(t, r.Passed())
	})

	t.Run("error", func(t *testing.T) {
		r := RunResult{ExitCode: 0, Error: assert.AnError}
		assert.False(t, r.Passed())
	})
}
