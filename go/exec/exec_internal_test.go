package command

import (
	"context"
	"testing"
	"time"

	core "github.com/dappcore/go"
)

// The extension helpers below carry the Windows resolution rules. They take
// the extension list as an argument rather than reading %PATHEXT%, so the
// semantics are pinned hermetically on every runner — no Windows box needed
// for the logic, and the CI lane proves the wiring.

func TestExecInternal_watchContext_Bad(t *testing.T) {
	// nil context is a no-op, no panic.
	c := &Cmd{}
	c.watchContext()

	// nil cmd with a live context is also a no-op.
	c = &Cmd{ctx: context.Background()}
	c.watchContext()
}

func TestExecInternal_watchContext_Good(t *testing.T) {
	// Cancelling the context kills a running process via the watch goroutine.
	ctx, cancel := context.WithCancel(context.Background())
	c := Command(ctx, "sleep", "30")

	if r := c.Start(); !r.OK {
		t.Fatalf("start failed: %v", r.Error())
	}
	cancel()

	// The watch goroutine should kill the process; Wait then returns.
	done := make(chan struct{})
	go func() {
		_ = c.cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("context cancellation should have killed the process")
	}
}

func TestExecInternal_commandContext_Good(t *testing.T) {
	// A resolvable name yields a command whose Path is the resolved binary
	// while Args[0] keeps the name the caller wrote.
	result := commandContext(context.Background(), "sh", "-c", "true")
	if !result.OK {
		t.Skipf("sh not resolvable on this platform: %v", result.Error())
	}
	cmd, ok := result.Value.(*core.Cmd)
	if !ok || cmd == nil {
		t.Fatalf("expected a *core.Cmd, got %T", result.Value)
	}
	if cmd.Args[0] != "sh" {
		t.Fatalf("expected first arg sh, got %v", cmd.Args[0])
	}
	if cmd.Path == "sh" {
		t.Fatal("expected Path to be resolved, not the bare name")
	}
}

func TestExecInternal_commandContext_Bad(t *testing.T) {
	// An unresolvable name FAILS rather than falling back to the raw name.
	// The old fallback handed os/exec a separator-free Path, which Windows
	// then resolves relative to Cmd.Dir — reporting the command as missing
	// from a working directory the caller never nominated as its home.
	result := commandContext(context.Background(), "definitely_not_a_real_binary_xyz")
	if result.OK {
		t.Fatalf("expected an unresolvable name to fail, got %v", result.Value)
	}
	if !core.Contains(result.Error(), "definitely_not_a_real_binary_xyz") {
		t.Fatalf("expected the failure to name the command, got %q", result.Error())
	}
}
