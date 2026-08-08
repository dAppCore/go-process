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

// The extension helpers below carry the Windows resolution rules. They take
// the extension list as an argument rather than reading %PATHEXT%, so the
// semantics are pinned hermetically on every runner — no Windows box needed
// for the logic, and the CI lane proves the wiring.

func TestExecInternal_parsePathExt_Good(t *testing.T) {
	got := parsePathExt(".COM;.EXE;.BAT;.CMD")
	want := []string{".com", ".exe", ".bat", ".cmd"}
	assertExtensions(t, got, want)

	// Order is significant — it is the order Windows tries them in.
	if got[0] != ".com" || got[1] != ".exe" {
		t.Fatalf("parsePathExt lost %%PATHEXT%% ordering: %v", got)
	}
}

func TestExecInternal_parsePathExt_Bad(t *testing.T) {
	// An unset or content-free value falls back to the set Windows assumes,
	// so a stripped environment still resolves the common executables.
	for _, value := range []string{"", "   ", ";;;", ".", "; . ;"} {
		assertExtensions(t, parsePathExt(value), []string{".com", ".exe", ".bat", ".cmd"})
	}
}

func TestExecInternal_parsePathExt_Ugly(t *testing.T) {
	// Dot-less entries are accepted (some installers write PATHEXT that way),
	// case is normalised, blanks and repeats are dropped, and a repeat does
	// not displace the first occurrence's position.
	assertExtensions(t,
		parsePathExt("EXE; .Bat ;;exe;.BAT;.ps1"),
		[]string{".exe", ".bat", ".ps1"})
}

func TestExecInternal_executableCandidates_Good(t *testing.T) {
	// A bare Windows name is tried once per extension, in order.
	assertExtensions(t,
		executableCandidates("C:/tools/git", []string{".com", ".exe"}),
		[]string{"C:/tools/git.com", "C:/tools/git.exe"})
}

func TestExecInternal_executableCandidates_Bad(t *testing.T) {
	// With no extensions in play — every POSIX case — the name stands alone
	// and is never suffixed.
	assertExtensions(t, executableCandidates("/usr/bin/git", nil), []string{"/usr/bin/git"})
	assertExtensions(t, executableCandidates("/usr/bin/git", []string{}), []string{"/usr/bin/git"})
}

func TestExecInternal_executableCandidates_Ugly(t *testing.T) {
	// A name that already carries a listed extension stands alone — it must
	// not become "git.exe.exe".
	assertExtensions(t,
		executableCandidates("C:/tools/git.exe", []string{".com", ".exe"}),
		[]string{"C:/tools/git.exe"})

	// Matching is case-insensitive, as Windows filenames are.
	assertExtensions(t,
		executableCandidates("C:/tools/GIT.EXE", []string{".com", ".exe"}),
		[]string{"C:/tools/GIT.EXE"})

	// An extension that is NOT listed is not an extension for this purpose:
	// "archive.tar" is a name to suffix, not an executable already.
	assertExtensions(t,
		executableCandidates("archive.tar", []string{".exe"}),
		[]string{"archive.tar.exe"})
}

func TestExecInternal_hasExecutableExtension_Good(t *testing.T) {
	if !hasExecutableExtension("git.exe", []string{".com", ".exe"}) {
		t.Fatal("expected git.exe to match .exe")
	}
	if !hasExecutableExtension("GIT.CMD", []string{".cmd"}) {
		t.Fatal("expected case-insensitive matching")
	}
}

func TestExecInternal_hasExecutableExtension_Bad(t *testing.T) {
	if hasExecutableExtension("git", []string{".com", ".exe"}) {
		t.Fatal("expected a bare name to match nothing")
	}
	if hasExecutableExtension("git.exe", nil) {
		t.Fatal("expected an empty extension list to match nothing")
	}
}

func TestExecInternal_containsSeparator_Good(t *testing.T) {
	// '/' is a directory component on every platform, so a name spelled with
	// it is a path to check directly rather than a name to hunt on PATH.
	if !containsSeparator("bin/tool") {
		t.Fatal("expected bin/tool to read as a path")
	}
	if !containsSeparator("/usr/bin/git") {
		t.Fatal("expected an absolute POSIX path to read as a path")
	}
}

func TestExecInternal_containsSeparator_Bad(t *testing.T) {
	if containsSeparator("git") {
		t.Fatal("expected a bare name to carry no directory component")
	}
	if containsSeparator("") {
		t.Fatal("expected an empty name to carry no directory component")
	}
}

// TestExecInternal_lookPathWith_Good is the receipt for the bug this fix
// exists for: on Windows a command is named without its extension, so a bare
// "tool" has to resolve to "tool.exe". Driving the extension list directly
// lets that be proven on a POSIX runner against a fixture PATH.
func TestExecInternal_lookPathWith_Good(t *testing.T) {
	dir := t.TempDir()
	// 0644, deliberately: under the Windows rule the mode bits are not the
	// question, and this file would be rejected by a mode&0111 test — which
	// is precisely why no Windows executable was ever found before.
	writeFixture(t, core.PathJoin(dir, "tool.exe"), 0o644)
	t.Setenv("PATH", dir)

	result := lookPathWith("tool", []string{".com", ".exe"})
	if !result.OK {
		t.Fatalf("expected bare \"tool\" to resolve to tool.exe, got %v", result.Error())
	}
	if want := core.PathJoin(dir, "tool.exe"); result.Value.(string) != want {
		t.Fatalf("resolved to %q, want %q", result.Value, want)
	}
}

// TestExecInternal_lookPathWith_Bad pins the POSIX contract as the same code
// path: with no extensions the name is used exactly as written, never
// suffixed, and the mode bits decide.
func TestExecInternal_lookPathWith_Bad(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, core.PathJoin(dir, "tool.exe"), 0o755)
	t.Setenv("PATH", dir)

	if result := lookPathWith("tool", nil); result.OK {
		t.Fatalf("expected no suffixing without extensions, resolved to %v", result.Value)
	}

	// The non-executable file is skipped even though its name matches.
	writeFixture(t, core.PathJoin(dir, "plain"), 0o644)
	if result := lookPathWith("plain", nil); result.OK {
		t.Fatalf("expected a 0644 file to be skipped, resolved to %v", result.Value)
	}
}

// TestExecInternal_lookPathWith_Ugly covers the order-sensitive and
// already-suffixed cases: %PATHEXT% order decides which of two matches wins,
// and a name given with its extension is not suffixed a second time.
func TestExecInternal_lookPathWith_Ugly(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, core.PathJoin(dir, "tool.com"), 0o644)
	writeFixture(t, core.PathJoin(dir, "tool.exe"), 0o644)
	t.Setenv("PATH", dir)

	// .com precedes .exe in the list, so .com wins.
	result := lookPathWith("tool", []string{".com", ".exe"})
	if !result.OK || result.Value.(string) != core.PathJoin(dir, "tool.com") {
		t.Fatalf("expected .com to win on list order, got %v (ok=%v)", result.Value, result.OK)
	}

	// Reversing the list reverses the winner — order is the whole rule.
	result = lookPathWith("tool", []string{".exe", ".com"})
	if !result.OK || result.Value.(string) != core.PathJoin(dir, "tool.exe") {
		t.Fatalf("expected .exe to win on reversed order, got %v (ok=%v)", result.Value, result.OK)
	}

	// A name already carrying a listed extension resolves as-is, not as
	// "tool.exe.exe".
	result = lookPathWith("tool.exe", []string{".com", ".exe"})
	if !result.OK || result.Value.(string) != core.PathJoin(dir, "tool.exe") {
		t.Fatalf("expected tool.exe to resolve as-is, got %v (ok=%v)", result.Value, result.OK)
	}

	// A path-qualified name takes the same extension treatment, and '/' reads
	// as a separator even under the Windows rules.
	result = lookPathWith(core.PathJoin(dir, "tool"), []string{".exe"})
	if !result.OK || result.Value.(string) != core.PathJoin(dir, "tool.exe") {
		t.Fatalf("expected a path-qualified bare name to gain .exe, got %v (ok=%v)", result.Value, result.OK)
	}
}

// TestExecInternal_isExecutableWith_Ugly pins the rule swap directly: the same
// 0644 file is not executable by the POSIX rule and is by the Windows one,
// while a directory is neither however it is spelled.
func TestExecInternal_isExecutableWith_Ugly(t *testing.T) {
	dir := t.TempDir()
	path := core.PathJoin(dir, "tool.exe")
	writeFixture(t, path, 0o644)

	if isExecutableWith(path, nil) {
		t.Fatal("expected a 0644 file to fail the POSIX mode test")
	}
	if !isExecutableWith(path, []string{".exe"}) {
		t.Fatal("expected a 0644 .exe to pass the Windows extension test")
	}

	// An extension not on the list is not executable under the Windows rule
	// even at 0755 — Windows does not consult the mode bits at all.
	other := core.PathJoin(dir, "notes.txt")
	writeFixture(t, other, 0o755)
	if isExecutableWith(other, []string{".exe"}) {
		t.Fatal("expected an unlisted extension to fail the Windows test")
	}

	// A directory named like an executable is still not one.
	subdir := core.PathJoin(dir, "bundle.exe")
	if r := core.MkdirAll(subdir, 0o755); !r.OK {
		t.Fatalf("mkdir failed: %v", r.Error())
	}
	if isExecutableWith(subdir, []string{".exe"}) {
		t.Fatal("expected a directory to be reported non-executable")
	}
}

// writeFixture creates a file with the given mode, failing the test if it
// cannot. WriteFile does not apply the mode to an existing file, so each
// fixture name is written once per test.
func writeFixture(t *testing.T, path string, mode core.FileMode) {
	t.Helper()
	if w := core.WriteFile(path, []byte("fixture"), mode); !w.OK {
		t.Fatalf("write %s: %v", path, w.Error())
	}
	if c := core.Chmod(path, mode); !c.OK {
		t.Fatalf("chmod %s: %v", path, c.Error())
	}
}

// assertExtensions compares two string slices element-wise, reporting the
// whole of both on mismatch so a reordering is readable.
func assertExtensions(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v (%d entries), want %v (%d entries)", got, len(got), want, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("entry %d: got %q, want %q (full: %v vs %v)", i, got[i], want[i], got, want)
		}
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
