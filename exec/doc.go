// Package command provides a small command wrapper around `os/exec` with
// structured logging hooks.
//
//	ctx := context.Background()
//	out, err := exec.Command(ctx, "echo", "hello").Output()
package command
