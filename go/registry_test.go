package process

import (
	"testing"
	"time"

	core "dappco.re/go"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	started := time.Now().UTC().Truncate(time.Second)
	entry := DaemonEntry{
		Code:    "myapp",
		Daemon:  "worker",
		PID:     core.Getpid(),
		Health:  "healthy",
		Project: "test-project",
		Binary:  "/usr/bin/worker",
		Started: started,
	}

	requireNoError(t, reg.Register(entry))

	got, ok := reg.Get("myapp", "worker")
	requireTrue(t, ok)
	assertEqual(t, "myapp", got.Code)
	assertEqual(t, "worker", got.Daemon)
	assertEqual(t, core.Getpid(), got.PID)
	assertEqual(t, "healthy", got.Health)
	assertEqual(t, "test-project", got.Project)
	assertEqual(t, "/usr/bin/worker", got.Binary)
	assertEqual(t, started, got.Started)
}

func TestRegistry_Unregister(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	entry := DaemonEntry{
		Code:   "myapp",
		Daemon: "server",
		PID:    core.Getpid(),
	}

	requireNoError(t, reg.Register(entry))

	// File should exist
	path := core.PathJoin(dir, "myapp-server.json")
	requireTrue(t, core.Stat(path).OK)

	requireNoError(t, reg.Unregister("myapp", "server"))

	// File should be gone
	stat := core.Stat(path)
	assertFalse(t, stat.OK)
	assertTrue(t, core.IsNotExist(stat.Value.(error)))
}

func TestRegistry_UnregisterMissingIsNoop(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	requireNoError(t, reg.Unregister("missing", "entry"))
}

func TestRegistry_List(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	requireNoError(t, reg.Register(DaemonEntry{Code: "app1", Daemon: "web", PID: core.Getpid()}))
	requireNoError(t, reg.Register(DaemonEntry{Code: "app2", Daemon: "api", PID: core.Getpid()}))

	entries := requireResultValue[[]DaemonEntry](t, reg.List())
	requireLen(t, entries, 2)
	assertEqual(t, "app1", entries[0].Code)
	assertEqual(t, "app2", entries[1].Code)
}

func TestRegistry_List_PrunesStale(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	requireNoError(t, reg.Register(DaemonEntry{Code: "dead", Daemon: "proc", PID: 999999999}))

	// File should exist before listing
	path := core.PathJoin(dir, "dead-proc.json")
	requireTrue(t, core.Stat(path).OK)

	entries := requireResultValue[[]DaemonEntry](t, reg.List())
	assertEmpty(t, entries)

	// Stale file should be removed
	stat := core.Stat(path)
	assertFalse(t, stat.OK)
	assertTrue(t, core.IsNotExist(stat.Value.(error)))
}

func TestRegistry_Get_NotFound(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	got, ok := reg.Get("nope", "missing")
	assertNil(t, got)
	assertFalse(t, ok)
}

func TestRegistry_CreatesDirectory(t *testing.T) {
	dir := core.PathJoin(t.TempDir(), "nested", "deep", "daemons")
	reg := NewRegistry(dir)

	requireNoError(t, reg.Register(DaemonEntry{Code: "app", Daemon: "srv", PID: core.Getpid()}))

	info := core.Stat(dir)
	requireTrue(t, info.OK)
	assertTrue(t, info.Value.(core.FsFileInfo).IsDir())
}

func TestDefaultRegistry(t *testing.T) {
	reg := DefaultRegistry()
	requireNotNil(t, reg)
	assertNotNil(t, reg)
}

func TestRegistry_NewRegistry_Good(t *testing.T) {
	dir := core.PathJoin(t.TempDir(), "daemons")
	reg := NewRegistry(dir)
	assertNotNil(t, reg)
	assertEqual(t, dir, reg.dir)
}

func TestRegistry_NewRegistry_Bad(t *testing.T) {
	reg := NewRegistry("")
	assertNotNil(t, reg)
	assertEqual(t, "", reg.dir)
}

func TestRegistry_NewRegistry_Ugly(t *testing.T) {
	dir := core.PathJoin(t.TempDir(), "nested", "daemons")
	reg := NewRegistry(dir)
	entries := requireResultValue[[]DaemonEntry](t, reg.List())
	assertEmpty(t, entries)
}

func TestRegistry_DefaultRegistry_Good(t *testing.T) {
	reg := DefaultRegistry()
	requireNotNil(t, reg)
	assertContains(t, reg.dir, ".core")
}

func TestRegistry_DefaultRegistry_Bad(t *testing.T) {
	reg := DefaultRegistry()
	requireNotNil(t, reg)
	assertContains(t, reg.dir, "daemons")
}

func TestRegistry_DefaultRegistry_Ugly(t *testing.T) {
	reg := DefaultRegistry()
	requireNotNil(t, reg)
	assertNotEmpty(t, reg.dir)
}

func TestRegistry_Registry_Register_Good(t *testing.T) {
	reg := NewRegistry(t.TempDir())
	entry := DaemonEntry{Code: "app", Daemon: "web", PID: core.Getpid()}
	requireNoError(t, reg.Register(entry))
	assertTrue(t, fileExists(core.PathJoin(reg.dir, "app-web.json")))
}

func TestRegistry_Registry_Register_Bad(t *testing.T) {
	reg := NewRegistry(t.TempDir())
	requireNoError(t, reg.Register(DaemonEntry{Code: "dead", Daemon: "web", PID: 0}))
	entries := requireResultValue[[]DaemonEntry](t, reg.List())
	assertEmpty(t, entries)
}

func TestRegistry_Registry_Register_Ugly(t *testing.T) {
	reg := NewRegistry(t.TempDir())
	requireNoError(t, reg.Register(DaemonEntry{Code: "app/name", Daemon: "worker/name", PID: core.Getpid()}))
	assertTrue(t, fileExists(core.PathJoin(reg.dir, "app-name-worker-name.json")))
}

func TestRegistry_Registry_Unregister_Good(t *testing.T) {
	reg := NewRegistry(t.TempDir())
	requireNoError(t, reg.Register(DaemonEntry{Code: "app", Daemon: "web", PID: core.Getpid()}))
	requireNoError(t, reg.Unregister("app", "web"))
	assertFalse(t, fileExists(core.PathJoin(reg.dir, "app-web.json")))
}

func TestRegistry_Registry_Unregister_Bad(t *testing.T) {
	reg := NewRegistry(t.TempDir())
	requireNoError(t, reg.Unregister("missing", "daemon"))
	assertNotNil(t, reg)
}

func TestRegistry_Registry_Unregister_Ugly(t *testing.T) {
	reg := NewRegistry(t.TempDir())
	requireNoError(t, reg.Register(DaemonEntry{Code: "app/name", Daemon: "worker/name", PID: core.Getpid()}))
	requireNoError(t, reg.Unregister("app/name", "worker/name"))
	assertFalse(t, fileExists(core.PathJoin(reg.dir, "app-name-worker-name.json")))
}

func TestRegistry_Registry_Get_Good(t *testing.T) {
	reg := NewRegistry(t.TempDir())
	requireNoError(t, reg.Register(DaemonEntry{Code: "app", Daemon: "web", PID: core.Getpid()}))
	entry, ok := reg.Get("app", "web")
	requireTrue(t, ok)
	assertEqual(t, "app", entry.Code)
}

func TestRegistry_Registry_Get_Bad(t *testing.T) {
	reg := NewRegistry(t.TempDir())
	entry, ok := reg.Get("missing", "web")
	assertNil(t, entry)
	assertFalse(t, ok)
}

func TestRegistry_Registry_Get_Ugly(t *testing.T) {
	reg := NewRegistry(t.TempDir())
	requireTrue(t, core.MkdirAll(reg.dir, 0755).OK)
	requireTrue(t, core.WriteFile(core.PathJoin(reg.dir, "bad-json.json"), []byte("{"), 0644).OK)
	entry, ok := reg.Get("bad", "json")
	assertNil(t, entry)
	assertFalse(t, ok)
}

func TestRegistry_Registry_List_Good(t *testing.T) {
	reg := NewRegistry(t.TempDir())
	requireNoError(t, reg.Register(DaemonEntry{Code: "app", Daemon: "web", PID: core.Getpid()}))
	entries := requireResultValue[[]DaemonEntry](t, reg.List())
	requireLen(t, entries, 1)
	assertEqual(t, "app", entries[0].Code)
}

func TestRegistry_Registry_List_Bad(t *testing.T) {
	reg := NewRegistry(t.TempDir())
	entries := requireResultValue[[]DaemonEntry](t, reg.List())
	assertEmpty(t, entries)
}

func TestRegistry_Registry_List_Ugly(t *testing.T) {
	reg := NewRegistry(t.TempDir())
	requireTrue(t, core.MkdirAll(reg.dir, 0755).OK)
	requireTrue(t, core.WriteFile(core.PathJoin(reg.dir, "bad-json.json"), []byte("{"), 0644).OK)
	entries := requireResultValue[[]DaemonEntry](t, reg.List())
	assertEmpty(t, entries)
}
