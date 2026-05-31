package command

import (
	"context"
	"testing"
	"time"

	core "dappco.re/go"
)

func TestExecInternal_lookPath_Good(t *testing.T) {
	// An absolute path to an executable resolves to itself.
	r := lookPath("/bin/sh")
	if !r.OK {
		t.Skip("/bin/sh not present on this platform")
	}
	if r.Value.(string) != "/bin/sh" {
		t.Fatalf("expected /bin/sh, got %v", r.Value)
	}

	// A bare name found on PATH resolves to an absolute path.
	r = lookPath("sh")
	if !r.OK {
		t.Skip("sh not on PATH")
	}
	if !core.Contains(r.Value.(string), "sh") {
		t.Fatalf("expected a path containing sh, got %v", r.Value)
	}
}

func TestExecInternal_lookPath_Bad(t *testing.T) {
	// Empty file name is rejected.
	if lookPath("").OK {
		t.Fatal("expected empty file name to fail")
	}

	// A path-qualified name that is not executable is rejected.
	if lookPath("/no/such/binary/here").OK {
		t.Fatal("expected missing path-qualified binary to fail")
	}
}

func TestExecInternal_lookPath_Ugly(t *testing.T) {
	// A bare name that is not on PATH is rejected after the PATH search.
	if lookPath("definitely_not_a_real_binary_xyz").OK {
		t.Fatal("expected unknown binary to fail PATH search")
	}

	// Empty PATH entries are treated as the current directory; with a name
	// that cannot be found anywhere, the search still terminates in failure.
	t.Setenv("PATH", "")
	if lookPath("definitely_not_a_real_binary_xyz").OK {
		t.Fatal("expected failure under empty PATH")
	}
}

func TestExecInternal_isExecutable_Good(t *testing.T) {
	if !isExecutable("/bin/sh") {
		t.Skip("/bin/sh not executable on this platform")
	}
}

func TestExecInternal_isExecutable_Bad(t *testing.T) {
	// A non-existent path is not executable.
	if isExecutable("/no/such/file") {
		t.Fatal("expected non-existent path to be non-executable")
	}
}

func TestExecInternal_isExecutable_Ugly(t *testing.T) {
	// A directory is not executable (even though it has the x bit).
	if isExecutable("/tmp") {
		t.Fatal("expected a directory to be reported non-executable")
	}

	// A regular, non-executable file is not executable.
	dir := t.TempDir()
	path := core.PathJoin(dir, "plain.txt")
	if w := core.WriteFile(path, []byte("data"), 0o600); !w.OK {
		t.Fatalf("write failed: %v", w.Error())
	}
	if isExecutable(path) {
		t.Fatal("expected a 0600 file to be non-executable")
	}
}

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
	// A resolvable name yields an absolute Path.
	cmd := commandContext(context.Background(), "sh", "-c", "true")
	if cmd == nil {
		t.Fatal("expected a command")
	}
	if cmd.Args[0] != "sh" {
		t.Fatalf("expected first arg sh, got %v", cmd.Args[0])
	}

	// An unresolvable name falls back to the raw name as Path.
	cmd = commandContext(context.Background(), "definitely_not_a_real_binary_xyz")
	if cmd.Path != "definitely_not_a_real_binary_xyz" {
		t.Fatalf("expected raw name fallback, got %v", cmd.Path)
	}
}
