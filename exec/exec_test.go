package exec_test

import (
	"context"
	"testing"

	"dappco.re/go/core"
	"dappco.re/go/core/process/exec"
)

// mockLogger captures log calls for testing
type mockLogger struct {
	debugCalls []logCall
	errorCalls []logCall
}

type logCall struct {
	msg     string
	keyvals []any
}

func (m *mockLogger) Debug(msg string, keyvals ...any) {
	m.debugCalls = append(m.debugCalls, logCall{msg, keyvals})
}

func (m *mockLogger) Error(msg string, keyvals ...any) {
	m.errorCalls = append(m.errorCalls, logCall{msg, keyvals})
}

func TestCommand_Run_Good(t *testing.T) {
	logger := &mockLogger{}
	ctx := context.Background()

	err := exec.Command(ctx, "echo", "hello").
		WithLogger(logger).
		Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(logger.debugCalls) != 1 {
		t.Fatalf("expected 1 debug call, got %d", len(logger.debugCalls))
	}
	if logger.debugCalls[0].msg != "executing command" {
		t.Errorf("expected msg 'executing command', got %q", logger.debugCalls[0].msg)
	}
	if len(logger.errorCalls) != 0 {
		t.Errorf("expected no error calls, got %d", len(logger.errorCalls))
	}
}

func TestCommand_Run_Bad(t *testing.T) {
	logger := &mockLogger{}
	ctx := context.Background()

	err := exec.Command(ctx, "false").
		WithLogger(logger).
		Run()
	if err == nil {
		t.Fatal("expected error")
	}

	if len(logger.debugCalls) != 1 {
		t.Fatalf("expected 1 debug call, got %d", len(logger.debugCalls))
	}
	if len(logger.errorCalls) != 1 {
		t.Fatalf("expected 1 error call, got %d", len(logger.errorCalls))
	}
	if logger.errorCalls[0].msg != "command failed" {
		t.Errorf("expected msg 'command failed', got %q", logger.errorCalls[0].msg)
	}
}

func TestCommand_Run_WithNilContext_Good(t *testing.T) {
	var ctx context.Context

	if err := exec.Command(ctx, "echo", "hello").Run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCommand_Output_Good(t *testing.T) {
	logger := &mockLogger{}
	ctx := context.Background()

	out, err := exec.Command(ctx, "echo", "test").
		WithLogger(logger).
		Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if core.Trim(string(out)) != "test" {
		t.Errorf("expected 'test', got %q", string(out))
	}
	if len(logger.debugCalls) != 1 {
		t.Errorf("expected 1 debug call, got %d", len(logger.debugCalls))
	}
}

func TestCommand_CombinedOutput_Good(t *testing.T) {
	logger := &mockLogger{}
	ctx := context.Background()

	out, err := exec.Command(ctx, "echo", "combined").
		WithLogger(logger).
		CombinedOutput()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if core.Trim(string(out)) != "combined" {
		t.Errorf("expected 'combined', got %q", string(out))
	}
	if len(logger.debugCalls) != 1 {
		t.Errorf("expected 1 debug call, got %d", len(logger.debugCalls))
	}
}

func TestNopLogger_Methods_Good(t *testing.T) {
	// Verify NopLogger doesn't panic
	var nop exec.NopLogger
	nop.Debug("msg", "key", "val")
	nop.Error("msg", "key", "val")
}

func TestLogger_SetDefault_Good(t *testing.T) {
	original := exec.DefaultLogger()
	defer exec.SetDefaultLogger(original)

	logger := &mockLogger{}
	exec.SetDefaultLogger(logger)

	if exec.DefaultLogger() != logger {
		t.Error("default logger not set correctly")
	}

	// Test nil resets to NopLogger
	exec.SetDefaultLogger(nil)
	if _, ok := exec.DefaultLogger().(exec.NopLogger); !ok {
		t.Error("expected NopLogger when setting nil")
	}
}

func TestCommand_UsesDefaultLogger_Good(t *testing.T) {
	original := exec.DefaultLogger()
	defer exec.SetDefaultLogger(original)

	logger := &mockLogger{}
	exec.SetDefaultLogger(logger)

	ctx := context.Background()
	_ = exec.Command(ctx, "echo", "test").Run()

	if len(logger.debugCalls) != 1 {
		t.Errorf("expected default logger to receive 1 debug call, got %d", len(logger.debugCalls))
	}
}

func TestCommand_WithDir_Good(t *testing.T) {
	ctx := context.Background()
	out, err := exec.Command(ctx, "pwd").
		WithDir("/tmp").
		WithLogger(&mockLogger{}).
		Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	trimmed := core.Trim(string(out))
	if trimmed != "/tmp" && trimmed != "/private/tmp" {
		t.Errorf("expected /tmp or /private/tmp, got %q", trimmed)
	}
}

func TestCommand_WithEnv_Good(t *testing.T) {
	ctx := context.Background()
	out, err := exec.Command(ctx, "sh", "-c", "echo $TEST_EXEC_VAR").
		WithEnv([]string{"TEST_EXEC_VAR=exec_val"}).
		WithLogger(&mockLogger{}).
		Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if core.Trim(string(out)) != "exec_val" {
		t.Errorf("expected 'exec_val', got %q", string(out))
	}
}

func TestCommand_WithStdinStdoutStderr_Good(t *testing.T) {
	ctx := context.Background()
	input := core.NewReader("piped input\n")
	stdout := core.NewBuilder()
	stderr := core.NewBuilder()

	err := exec.Command(ctx, "cat").
		WithStdin(input).
		WithStdout(stdout).
		WithStderr(stderr).
		WithLogger(&mockLogger{}).
		Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if core.Trim(stdout.String()) != "piped input" {
		t.Errorf("expected 'piped input', got %q", stdout.String())
	}
}

func TestRunQuiet_Command_Good(t *testing.T) {
	ctx := context.Background()
	err := exec.RunQuiet(ctx, "echo", "quiet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunQuiet_Command_Bad(t *testing.T) {
	ctx := context.Background()
	err := exec.RunQuiet(ctx, "sh", "-c", "echo fail >&2; exit 1")
	if err == nil {
		t.Fatal("expected error")
	}
}
