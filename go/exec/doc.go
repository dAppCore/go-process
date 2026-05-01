// Package command wraps stdlib process spawning with a Result-shaped API
// and structured logging hooks.
//
//	ctx := context.Background()
//	out, err := exec.Command(ctx, "echo", "hello").Output()
package command
