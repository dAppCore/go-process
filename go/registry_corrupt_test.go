package process

import (
	"testing"

	core "dappco.re/go"
)

func TestRegistryCorrupt_Get_PrunesCorruptJSON_Ugly(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	// Write a file at the expected entry path that is not valid JSON. Get
	// must fail to unmarshal, remove the corrupt file, and report not-found.
	path := reg.entryPath("app", "serve")
	requireTrue(t, core.MkdirAll(dir, 0o755).OK)
	requireTrue(t, core.WriteFile(path, []byte("{not valid json"), 0o644).OK)

	entry, ok := reg.Get("app", "serve")
	assertFalse(t, ok)
	assertNil(t, entry)

	// The corrupt file has been pruned.
	assertFalse(t, fileExists(path))
}

func TestRegistryCorrupt_List_SkipsCorruptJSON_Ugly(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)
	requireTrue(t, core.MkdirAll(dir, 0o755).OK)

	// One valid alive entry plus one corrupt file.
	requireNoError(t, reg.Register(DaemonEntry{
		Code:   "live",
		Daemon: "serve",
		PID:    core.Getpid(),
	}))
	corrupt := core.PathJoin(dir, "broken-entry.json")
	requireTrue(t, core.WriteFile(corrupt, []byte("}{garbage"), 0o644).OK)

	result := reg.List()
	requireTrue(t, result.OK)
	entries := result.Value.([]DaemonEntry)

	// The corrupt file is skipped (and pruned); only the live entry survives.
	requireLen(t, entries, 1)
	assertEqual(t, "live", entries[0].Code)
	assertFalse(t, fileExists(corrupt))
}

func TestRegistryCorrupt_List_PrunesStaleAndCorrupt_Ugly(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	// A stale (dead-PID) entry alongside a corrupt file: both removed, List
	// returns an empty (non-nil) slice.
	requireNoError(t, reg.Register(DaemonEntry{
		Code:   "ghost",
		Daemon: "serve",
		PID:    2147483646,
	}))
	corrupt := core.PathJoin(dir, "another-broken.json")
	requireTrue(t, core.WriteFile(corrupt, []byte("not json at all"), 0o644).OK)

	result := reg.List()
	requireTrue(t, result.OK)
	entries := result.Value.([]DaemonEntry)
	assertLen(t, entries, 0)
}

func TestRegistryCorrupt_Register_PreservesStarted_Good(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)

	// A pre-set Started timestamp is preserved (the zero-check branch is
	// false), distinct from the auto-stamp path.
	entry := DaemonEntry{Code: "app", Daemon: "serve", PID: core.Getpid()}
	entry.Started = entry.Started.AddDate(2020, 0, 0) // any non-zero time
	requireNoError(t, reg.Register(entry))

	got, ok := reg.Get("app", "serve")
	requireTrue(t, ok)
	assertEqual(t, entry.Started, got.Started)
}
