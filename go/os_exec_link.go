package process

import (
	"context"

	core "dappco.re/go"
	"dappco.re/go/process/internal/lookpath"
)

// commandContext resolves name to a concrete executable and builds the handle
// for it.
//
// A resolution failure is RECORDED ON THE HANDLE rather than discarded, exactly
// as exec.Command does with its own LookPath error: Cmd.Err is returned by
// Start without running anything. The old code dropped the failure and left
// Path as the bare name, which is not a harmless fallback on Windows — with
// Dir set, Cmd.Start resolves a separator-free Path relative to Dir, so an
// unresolved "git" surfaced as `exec: "C:\...\some work dir\git"`, naming a
// working directory nobody asked about instead of PATH.
//
// Deferring to Start rather than returning the error here keeps the callers'
// failure handling intact — Service.start still broadcasts its exited event,
// and Program.RunDir still wraps the error in its own operation.
func commandContext(ctx context.Context, name string, arg ...string) *core.Cmd {
	cmd := &core.Cmd{Args: append([]string{name}, arg...)}
	resolved := lookpath.Look(name)
	if !resolved.OK {
		cmd.Path = name
		if failure, ok := resolved.Value.(error); ok {
			cmd.Err = failure
		} else {
			cmd.Err = core.E("commandContext", core.Sprintf("%q: not found in PATH", name), nil)
		}
		return cmd
	}
	cmd.Path = resolved.Value.(string)
	return cmd
}
