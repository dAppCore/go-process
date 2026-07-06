package process

// Platform helpers centralize process-group and signal behavior that differs
// between Unix and Windows. Unix implementations preserve POSIX group
// semantics; Windows implementations provide Phase 1 best-effort behavior.
