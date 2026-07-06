package process

import (
	"testing"
	"time"

	core "dappco.re/go"
)

func TestHelpers_cancelledRunResult_Good(t *testing.T) {
	spec := RunSpec{Name: "build", Command: "echo"}

	// With no upstream error the result reports a context-cancelled failure.
	res := cancelledRunResult("Runner.RunAll", spec, nil)
	assertTrue(t, res.Skipped)
	assertEqual(t, 1, res.ExitCode)
	requireNotNil(t, res.Error)
	assertContains(t, res.Error.Error(), "context cancelled")
	assertEqual(t, "build", res.Name)
}

func TestHelpers_cancelledRunResult_Bad(t *testing.T) {
	spec := RunSpec{Name: "test"}

	// With an upstream error, skippedRunResult already sets Error, so
	// cancelledRunResult preserves the skipped reason rather than overwriting.
	cause := core.NewError("dependency failed")
	res := cancelledRunResult("Runner.RunAll", spec, cause)
	assertTrue(t, res.Skipped)
	assertEqual(t, 1, res.ExitCode)
	requireNotNil(t, res.Error)
	assertContains(t, res.Error.Error(), "skipped")
}

func TestHelpers_skippedRunResult_Ugly(t *testing.T) {
	spec := RunSpec{Name: "lint"}

	// A nil error yields a skipped result with no error and zero exit code.
	res := skippedRunResult("Runner.RunAll", spec, nil)
	assertTrue(t, res.Skipped)
	assertEqual(t, 0, res.ExitCode)
	assertNil(t, res.Error)
}

func TestHelpers_sortProcesses_Good(t *testing.T) {
	base := time.Now()
	a := &Process{ID: "a", StartedAt: base.Add(2 * time.Second)}
	b := &Process{ID: "b", StartedAt: base.Add(1 * time.Second)}
	c := &Process{ID: "c", StartedAt: base}

	procs := []*Process{a, b, c}
	sortProcesses(procs)

	// Sorted oldest-first by StartedAt.
	assertEqual(t, "c", procs[0].ID)
	assertEqual(t, "b", procs[1].ID)
	assertEqual(t, "a", procs[2].ID)
}

func TestHelpers_sortProcesses_Bad(t *testing.T) {
	// Equal StartedAt falls back to ID ordering (both directions exercised).
	ts := time.Now()
	x := &Process{ID: "x", StartedAt: ts}
	m := &Process{ID: "m", StartedAt: ts}
	a := &Process{ID: "a", StartedAt: ts}

	procs := []*Process{x, m, a}
	sortProcesses(procs)

	assertEqual(t, "a", procs[0].ID)
	assertEqual(t, "m", procs[1].ID)
	assertEqual(t, "x", procs[2].ID)
}

func TestHelpers_sortProcesses_Ugly(t *testing.T) {
	// Empty and single-element slices are stable no-ops.
	var empty []*Process
	sortProcesses(empty)
	assertLen(t, empty, 0)

	single := []*Process{{ID: "only", StartedAt: time.Now()}}
	sortProcesses(single)
	assertEqual(t, "only", single[0].ID)

	// Identical IDs and timestamps compare equal (return 0 branch).
	ts := time.Now()
	dup := []*Process{{ID: "same", StartedAt: ts}, {ID: "same", StartedAt: ts}}
	sortProcesses(dup)
	assertEqual(t, "same", dup[0].ID)
}

func TestHelpers_trimRightSpace_Good(t *testing.T) {
	assertEqual(t, "hello", trimRightSpace("hello   \n\t "))
	assertEqual(t, "hello", trimRightSpace("hello"))
}

func TestHelpers_trimRightSpace_Bad(t *testing.T) {
	// All-whitespace trims to empty.
	assertEqual(t, "", trimRightSpace("   \n\t"))
	assertEqual(t, "", trimRightSpace(""))
}

func TestHelpers_trimRightSpace_Ugly(t *testing.T) {
	// Multi-byte trailing rune is preserved; trailing space after it trimmed.
	assertEqual(t, "café", trimRightSpace("café  "))
	// Multi-byte content with no trailing whitespace is unchanged.
	assertEqual(t, "日本語", trimRightSpace("日本語"))
}

func TestHelpers_utf8LastRuneInString_Good(t *testing.T) {
	// ASCII single byte.
	r, size := utf8LastRuneInString("abc")
	assertEqual(t, 'c', r)
	assertEqual(t, 1, size)

	// Multi-byte trailing rune reports its full width.
	r, size = utf8LastRuneInString("café")
	assertEqual(t, 'é', r)
	assertEqual(t, 2, size)
}

func TestHelpers_utf8LastRuneInString_Bad(t *testing.T) {
	// Empty string reports the zero rune and zero width.
	r, size := utf8LastRuneInString("")
	assertEqual(t, rune(0), r)
	assertEqual(t, 0, size)
}

func TestHelpers_utf8LastRuneInString_Ugly(t *testing.T) {
	// A three-byte rune (e.g. a CJK character) reports width 3.
	r, size := utf8LastRuneInString("a語")
	assertEqual(t, '語', r)
	assertEqual(t, 3, size)
}
