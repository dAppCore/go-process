package process

import (
	"context"
	"testing"

	framework "dappco.re/go"
)

func newTestRunner(t *testing.T) *Runner {
	t.Helper()

	c := framework.New()
	factory := NewService(Options{})
	raw, err := factory(c)
	requireNoError(t, err)

	return NewRunner(raw.(*Service))
}

func TestRunner_RunSequential(t *testing.T) {
	t.Run("all pass", func(t *testing.T) {
		runner := newTestRunner(t)

		result, err := runner.RunSequential(context.Background(), []RunSpec{
			{Name: "first", Command: "echo", Args: []string{"1"}},
			{Name: "second", Command: "echo", Args: []string{"2"}},
			{Name: "third", Command: "echo", Args: []string{"3"}},
		})
		requireNoError(t, err)

		assertTrue(t, result.Success())
		assertEqual(t, 3, result.Passed)
		assertEqual(t, 0, result.Failed)
		assertEqual(t, 0, result.Skipped)
	})

	t.Run("stops on failure", func(t *testing.T) {
		runner := newTestRunner(t)

		result, err := runner.RunSequential(context.Background(), []RunSpec{
			{Name: "first", Command: "echo", Args: []string{"1"}},
			{Name: "fails", Command: "sh", Args: []string{"-c", "exit 1"}},
			{Name: "third", Command: "echo", Args: []string{"3"}},
		})
		requireNoError(t, err)

		assertFalse(t, result.Success())
		assertEqual(t, 1, result.Passed)
		assertEqual(t, 1, result.Failed)
		assertEqual(t, 1, result.Skipped)
		requireLen(t, result.Results, 3)
		assertEqual(t, 0, result.Results[0].ExitCode)
		assertNoError(t, result.Results[0].Error)
		assertEqual(t, 1, result.Results[1].ExitCode)
		assertNoError(t, result.Results[1].Error)
		assertTrue(t, result.Results[2].Skipped)
	})

	t.Run("allow failure continues", func(t *testing.T) {
		runner := newTestRunner(t)

		result, err := runner.RunSequential(context.Background(), []RunSpec{
			{Name: "first", Command: "echo", Args: []string{"1"}},
			{Name: "fails", Command: "sh", Args: []string{"-c", "exit 1"}, AllowFailure: true},
			{Name: "third", Command: "echo", Args: []string{"3"}},
		})
		requireNoError(t, err)

		// Still counts as failed but pipeline continues
		assertEqual(t, 2, result.Passed)
		assertEqual(t, 1, result.Failed)
		assertEqual(t, 0, result.Skipped)
	})
}

func TestRunner_RunParallel(t *testing.T) {
	t.Run("all run concurrently", func(t *testing.T) {
		runner := newTestRunner(t)

		result, err := runner.RunParallel(context.Background(), []RunSpec{
			{Name: "first", Command: "echo", Args: []string{"1"}},
			{Name: "second", Command: "echo", Args: []string{"2"}},
			{Name: "third", Command: "echo", Args: []string{"3"}},
		})
		requireNoError(t, err)

		assertTrue(t, result.Success())
		assertEqual(t, 3, result.Passed)
		assertLen(t, result.Results, 3)
	})

	t.Run("failure doesnt stop others", func(t *testing.T) {
		runner := newTestRunner(t)

		result, err := runner.RunParallel(context.Background(), []RunSpec{
			{Name: "first", Command: "echo", Args: []string{"1"}},
			{Name: "fails", Command: "sh", Args: []string{"-c", "exit 1"}},
			{Name: "third", Command: "echo", Args: []string{"3"}},
		})
		requireNoError(t, err)

		assertFalse(t, result.Success())
		assertEqual(t, 2, result.Passed)
		assertEqual(t, 1, result.Failed)
	})
}

func TestRunner_RunAll(t *testing.T) {
	t.Run("respects dependencies", func(t *testing.T) {
		runner := newTestRunner(t)

		result, err := runner.RunAll(context.Background(), []RunSpec{
			{Name: "third", Command: "echo", Args: []string{"3"}, After: []string{"second"}},
			{Name: "first", Command: "echo", Args: []string{"1"}},
			{Name: "second", Command: "echo", Args: []string{"2"}, After: []string{"first"}},
		})
		requireNoError(t, err)

		assertTrue(t, result.Success())
		assertEqual(t, 3, result.Passed)
	})

	t.Run("skips dependents on failure", func(t *testing.T) {
		runner := newTestRunner(t)

		result, err := runner.RunAll(context.Background(), []RunSpec{
			{Name: "first", Command: "sh", Args: []string{"-c", "exit 1"}},
			{Name: "second", Command: "echo", Args: []string{"2"}, After: []string{"first"}},
			{Name: "third", Command: "echo", Args: []string{"3"}, After: []string{"second"}},
		})
		requireNoError(t, err)

		assertFalse(t, result.Success())
		assertEqual(t, 0, result.Passed)
		assertEqual(t, 1, result.Failed)
		assertEqual(t, 2, result.Skipped)
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
		requireNoError(t, err)

		assertTrue(t, result.Success())
		assertEqual(t, 4, result.Passed)
	})

	t.Run("preserves input order", func(t *testing.T) {
		runner := newTestRunner(t)

		specs := []RunSpec{
			{Name: "third", Command: "echo", Args: []string{"3"}, After: []string{"second"}},
			{Name: "first", Command: "echo", Args: []string{"1"}},
			{Name: "second", Command: "echo", Args: []string{"2"}, After: []string{"first"}},
		}

		result, err := runner.RunAll(context.Background(), specs)
		requireNoError(t, err)

		requireLen(t, result.Results, len(specs))
		for i, res := range result.Results {
			assertEqual(t, specs[i].Name, res.Name)
		}
	})
}

func TestRunner_RunAll_CircularDeps(t *testing.T) {
	t.Run("circular dependency is skipped with error", func(t *testing.T) {
		runner := newTestRunner(t)

		result, err := runner.RunAll(context.Background(), []RunSpec{
			{Name: "a", Command: "echo", Args: []string{"a"}, After: []string{"b"}},
			{Name: "b", Command: "echo", Args: []string{"b"}, After: []string{"a"}},
		})
		requireNoError(t, err)

		assertTrue(t, result.Success())
		assertEqual(t, 0, result.Failed)
		assertEqual(t, 2, result.Skipped)
		for _, res := range result.Results {
			assertTrue(t, res.Skipped)
			assertEqual(t, 0, res.ExitCode)
			assertError(t, res.Error)
		}
	})

	t.Run("missing dependency is skipped with error", func(t *testing.T) {
		runner := newTestRunner(t)

		result, err := runner.RunAll(context.Background(), []RunSpec{
			{Name: "a", Command: "echo", Args: []string{"a"}, After: []string{"missing"}},
		})
		requireNoError(t, err)

		assertTrue(t, result.Success())
		assertEqual(t, 0, result.Failed)
		assertEqual(t, 1, result.Skipped)
		requireLen(t, result.Results, 1)
		assertTrue(t, result.Results[0].Skipped)
		assertEqual(t, 0, result.Results[0].ExitCode)
		assertError(t, result.Results[0].Error)
	})
}

func TestRunner_ContextCancellation(t *testing.T) {
	t.Run("run sequential skips pending specs", func(t *testing.T) {
		runner := newTestRunner(t)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		result, err := runner.RunSequential(ctx, []RunSpec{
			{Name: "first", Command: "echo", Args: []string{"1"}},
			{Name: "second", Command: "echo", Args: []string{"2"}},
		})
		requireNoError(t, err)

		assertEqual(t, 0, result.Passed)
		assertEqual(t, 0, result.Failed)
		assertEqual(t, 2, result.Skipped)
		requireLen(t, result.Results, 2)
		for _, res := range result.Results {
			assertTrue(t, res.Skipped)
			assertEqual(t, 1, res.ExitCode)
			assertError(t, res.Error)
			assertContains(t, res.Error.Error(), "context canceled")
		}
	})

	t.Run("run all skips pending specs", func(t *testing.T) {
		runner := newTestRunner(t)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		result, err := runner.RunAll(ctx, []RunSpec{
			{Name: "first", Command: "echo", Args: []string{"1"}},
			{Name: "second", Command: "echo", Args: []string{"2"}, After: []string{"first"}},
		})
		requireNoError(t, err)

		assertEqual(t, 0, result.Passed)
		assertEqual(t, 0, result.Failed)
		assertEqual(t, 2, result.Skipped)
		requireLen(t, result.Results, 2)
		for _, res := range result.Results {
			assertTrue(t, res.Skipped)
			assertEqual(t, 1, res.ExitCode)
			assertError(t, res.Error)
			assertContains(t, res.Error.Error(), "context canceled")
		}
	})
}

func TestRunResult_Passed(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		r := RunResult{ExitCode: 0}
		assertTrue(t, r.Passed())
	})

	t.Run("non-zero exit", func(t *testing.T) {
		r := RunResult{ExitCode: 1}
		assertFalse(t, r.Passed())
	})

	t.Run("skipped", func(t *testing.T) {
		r := RunResult{ExitCode: 0, Skipped: true}
		assertFalse(t, r.Passed())
	})

	t.Run("error", func(t *testing.T) {
		r := RunResult{ExitCode: 0, Error: errSentinel}
		assertFalse(t, r.Passed())
	})
}

func TestRunner_NilService(t *testing.T) {
	runner := NewRunner(nil)

	_, err := runner.RunAll(context.Background(), nil)
	requireError(t, err)
	assertErrorIs(t, err, ErrRunnerNoService)

	_, err = runner.RunSequential(context.Background(), nil)
	requireError(t, err)
	assertErrorIs(t, err, ErrRunnerNoService)

	_, err = runner.RunParallel(context.Background(), nil)
	requireError(t, err)
	assertErrorIs(t, err, ErrRunnerNoService)
}

func TestRunner_NilContext(t *testing.T) {
	runner := newTestRunner(t)

	_, err := runner.RunAll(nil, nil)
	requireError(t, err)
	assertErrorIs(t, err, ErrRunnerContextRequired)

	_, err = runner.RunSequential(nil, nil)
	requireError(t, err)
	assertErrorIs(t, err, ErrRunnerContextRequired)

	_, err = runner.RunParallel(nil, nil)
	requireError(t, err)
	assertErrorIs(t, err, ErrRunnerContextRequired)
}

func TestRunner_InvalidSpecNames(t *testing.T) {
	runner := newTestRunner(t)

	t.Run("rejects empty names", func(t *testing.T) {
		_, err := runner.RunSequential(context.Background(), []RunSpec{
			{Name: "", Command: "echo", Args: []string{"a"}},
		})
		requireError(t, err)
		assertErrorIs(t, err, ErrRunnerInvalidSpecName)
	})

	t.Run("rejects empty dependency names", func(t *testing.T) {
		_, err := runner.RunAll(context.Background(), []RunSpec{
			{Name: "one", Command: "echo", Args: []string{"a"}, After: []string{""}},
		})
		requireError(t, err)
		assertErrorIs(t, err, ErrRunnerInvalidDependencyName)
	})

	t.Run("rejects duplicated dependency names", func(t *testing.T) {
		_, err := runner.RunAll(context.Background(), []RunSpec{
			{Name: "one", Command: "echo", Args: []string{"a"}, After: []string{"two", "two"}},
		})
		requireError(t, err)
		assertErrorIs(t, err, ErrRunnerInvalidDependencyName)
	})

	t.Run("rejects self dependency", func(t *testing.T) {
		_, err := runner.RunAll(context.Background(), []RunSpec{
			{Name: "one", Command: "echo", Args: []string{"a"}, After: []string{"one"}},
		})
		requireError(t, err)
		assertErrorIs(t, err, ErrRunnerInvalidDependencyName)
	})

	t.Run("rejects duplicate names", func(t *testing.T) {
		_, err := runner.RunAll(context.Background(), []RunSpec{
			{Name: "same", Command: "echo", Args: []string{"a"}},
			{Name: "same", Command: "echo", Args: []string{"b"}},
		})
		requireError(t, err)
		assertErrorIs(t, err, ErrRunnerInvalidSpecName)
	})

	t.Run("rejects duplicate names in parallel mode", func(t *testing.T) {
		_, err := runner.RunParallel(context.Background(), []RunSpec{
			{Name: "one", Command: "echo", Args: []string{"a"}},
			{Name: "one", Command: "echo", Args: []string{"b"}},
		})
		requireError(t, err)
		assertErrorIs(t, err, ErrRunnerInvalidSpecName)
	})
}

func TestRunner_NewRunner_Good(t *testing.T) {
	svc, _ := newTestService(t)
	runner := NewRunner(svc)
	assertNotNil(t, runner)
	assertEqual(t, svc, runner.service)
}

func TestRunner_NewRunner_Bad(t *testing.T) {
	runner := NewRunner(nil)
	result, err := runner.RunSequential(context.Background(), nil)
	assertNil(t, result)
	assertErrorIs(t, err, ErrRunnerNoService)
}

func TestRunner_NewRunner_Ugly(t *testing.T) {
	var svc *Service
	runner := NewRunner(svc)
	assertNotNil(t, runner)
	assertNil(t, runner.service)
}

func TestRunner_RunResult_Passed_Good(t *testing.T) {
	result := RunResult{ExitCode: 0}
	got := result.Passed()
	assertTrue(t, got)
	assertEqual(t, 0, result.ExitCode)
}

func TestRunner_RunResult_Passed_Bad(t *testing.T) {
	result := RunResult{ExitCode: 1}
	got := result.Passed()
	assertFalse(t, got)
	assertEqual(t, 1, result.ExitCode)
}

func TestRunner_RunResult_Passed_Ugly(t *testing.T) {
	result := RunResult{Skipped: true}
	got := result.Passed()
	assertFalse(t, got)
	assertTrue(t, result.Skipped)
}

func TestRunner_RunAllResult_Success_Good(t *testing.T) {
	result := RunAllResult{Failed: 0, Passed: 2}
	got := result.Success()
	assertTrue(t, got)
	assertEqual(t, 2, result.Passed)
}

func TestRunner_RunAllResult_Success_Bad(t *testing.T) {
	result := RunAllResult{Failed: 1}
	got := result.Success()
	assertFalse(t, got)
	assertEqual(t, 1, result.Failed)
}

func TestRunner_RunAllResult_Success_Ugly(t *testing.T) {
	result := RunAllResult{}
	got := result.Success()
	assertTrue(t, got)
	assertEqual(t, 0, result.Failed)
}

func TestRunner_Runner_RunAll_Good(t *testing.T) {
	svc, _ := newTestService(t)
	runner := NewRunner(svc)
	result, err := runner.RunAll(context.Background(), []RunSpec{{Name: "echo", Command: "echo", Args: []string{"ok"}}})
	requireNoError(t, err)
	requireLen(t, result.Results, 1)
	assertTrue(t, result.Success())
}

func TestRunner_Runner_RunAll_Bad(t *testing.T) {
	runner := NewRunner(nil)
	result, err := runner.RunAll(context.Background(), nil)
	assertNil(t, result)
	assertErrorIs(t, err, ErrRunnerNoService)
}

func TestRunner_Runner_RunAll_Ugly(t *testing.T) {
	svc, _ := newTestService(t)
	runner := NewRunner(svc)
	result, err := runner.RunAll(context.Background(), []RunSpec{{Name: "self", Command: "echo", After: []string{"self"}}})
	assertNil(t, result)
	assertErrorIs(t, err, ErrRunnerInvalidDependencyName)
}

func TestRunner_Runner_RunSequential_Good(t *testing.T) {
	svc, _ := newTestService(t)
	runner := NewRunner(svc)
	result, err := runner.RunSequential(context.Background(), []RunSpec{{Name: "echo", Command: "echo", Args: []string{"ok"}}})
	requireNoError(t, err)
	requireLen(t, result.Results, 1)
	assertTrue(t, result.Success())
}

func TestRunner_Runner_RunSequential_Bad(t *testing.T) {
	runner := NewRunner(nil)
	result, err := runner.RunSequential(context.Background(), nil)
	assertNil(t, result)
	assertErrorIs(t, err, ErrRunnerNoService)
}

func TestRunner_Runner_RunSequential_Ugly(t *testing.T) {
	svc, _ := newTestService(t)
	runner := NewRunner(svc)
	result, err := runner.RunSequential(nil, []RunSpec{{Name: "echo", Command: "echo"}})
	assertNil(t, result)
	assertErrorIs(t, err, ErrRunnerContextRequired)
}

func TestRunner_Runner_RunParallel_Good(t *testing.T) {
	svc, _ := newTestService(t)
	runner := NewRunner(svc)
	result, err := runner.RunParallel(context.Background(), []RunSpec{{Name: "echo", Command: "echo", Args: []string{"ok"}}})
	requireNoError(t, err)
	requireLen(t, result.Results, 1)
	assertTrue(t, result.Success())
}

func TestRunner_Runner_RunParallel_Bad(t *testing.T) {
	runner := NewRunner(nil)
	result, err := runner.RunParallel(context.Background(), nil)
	assertNil(t, result)
	assertErrorIs(t, err, ErrRunnerNoService)
}

func TestRunner_Runner_RunParallel_Ugly(t *testing.T) {
	svc, _ := newTestService(t)
	runner := NewRunner(svc)
	result, err := runner.RunParallel(context.Background(), []RunSpec{{Name: "", Command: "echo"}})
	assertNil(t, result)
	assertErrorIs(t, err, ErrRunnerInvalidSpecName)
}
