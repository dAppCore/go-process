package exec_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	// Note: AX-6 — internal concurrency primitive; structural per RFC §2
	"sync"
	"testing"
	"time"

	"dappco.re/go/process/exec"
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

func TestCommand_Run_Good_LogsDebug(t *testing.T) {
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

func TestCommand_Run_Bad_LogsError(t *testing.T) {
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

func TestCommand_Output_Good(t *testing.T) {
	logger := &mockLogger{}
	ctx := context.Background()

	out, err := exec.Command(ctx, "echo", "test").
		WithLogger(logger).
		Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(string(out)) != "test" {
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
	if strings.TrimSpace(string(out)) != "combined" {
		t.Errorf("expected 'combined', got %q", string(out))
	}
	if len(logger.debugCalls) != 1 {
		t.Errorf("expected 1 debug call, got %d", len(logger.debugCalls))
	}
}

func TestNopLogger(t *testing.T) {
	// Verify NopLogger doesn't panic
	var nop exec.NopLogger
	nop.Debug("msg", "key", "val")
	nop.Error("msg", "key", "val")
}

func TestSetDefaultLogger(t *testing.T) {
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

func TestDefaultLogger_IsConcurrentSafe(t *testing.T) {
	original := exec.DefaultLogger()
	defer exec.SetDefaultLogger(original)

	logger := &mockLogger{}

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			exec.SetDefaultLogger(logger)
		}()
		go func() {
			defer wg.Done()
			_ = exec.DefaultLogger()
		}()
	}
	wg.Wait()

	if exec.DefaultLogger() == nil {
		t.Fatal("expected non-nil default logger")
	}
}

func TestCommand_UsesDefaultLogger(t *testing.T) {
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

func TestCommand_WithDir(t *testing.T) {
	ctx := context.Background()
	out, err := exec.Command(ctx, "pwd").
		WithDir("/tmp").
		WithLogger(&mockLogger{}).
		Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed != "/tmp" && trimmed != "/private/tmp" {
		t.Errorf("expected /tmp or /private/tmp, got %q", trimmed)
	}
}

func TestCommand_WithEnv(t *testing.T) {
	ctx := context.Background()
	out, err := exec.Command(ctx, "sh", "-c", "echo $TEST_EXEC_VAR").
		WithEnv([]string{"TEST_EXEC_VAR=exec_val"}).
		WithLogger(&mockLogger{}).
		Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(string(out)) != "exec_val" {
		t.Errorf("expected 'exec_val', got %q", string(out))
	}
}

func TestCommand_WithStdinStdoutStderr(t *testing.T) {
	ctx := context.Background()
	input := strings.NewReader("piped input\n")
	var stdout, stderr strings.Builder

	err := exec.Command(ctx, "cat").
		WithStdin(input).
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithLogger(&mockLogger{}).
		Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(stdout.String()) != "piped input" {
		t.Errorf("expected 'piped input', got %q", stdout.String())
	}
}

func TestCommand_Run_Background(t *testing.T) {
	logger := &mockLogger{}
	ctx := context.Background()
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker.txt")

	start := time.Now()
	err := exec.Command(ctx, "sh", "-c", fmt.Sprintf("sleep 0.2; printf done > %q", marker)).
		WithBackground(true).
		WithLogger(logger).
		Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("background run took too long: %s", elapsed)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		data, readErr := os.ReadFile(marker)
		if readErr == nil && strings.TrimSpace(string(data)) == "done" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("background command did not create marker file")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestCommand_NilContextRejected(t *testing.T) {
	t.Run("start", func(t *testing.T) {
		err := exec.Command(nil, "echo", "test").Start()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, exec.ErrCommandContextRequired) {
			t.Fatalf("expected ErrCommandContextRequired, got %v", err)
		}
	})

	t.Run("run", func(t *testing.T) {
		err := exec.Command(nil, "echo", "test").Run()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, exec.ErrCommandContextRequired) {
			t.Fatalf("expected ErrCommandContextRequired, got %v", err)
		}
	})

	t.Run("output", func(t *testing.T) {
		_, err := exec.Command(nil, "echo", "test").Output()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, exec.ErrCommandContextRequired) {
			t.Fatalf("expected ErrCommandContextRequired, got %v", err)
		}
	})

	t.Run("combined output", func(t *testing.T) {
		_, err := exec.Command(nil, "echo", "test").CombinedOutput()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, exec.ErrCommandContextRequired) {
			t.Fatalf("expected ErrCommandContextRequired, got %v", err)
		}
	})
}

func TestCommand_Output_BackgroundRejected(t *testing.T) {
	ctx := context.Background()

	_, err := exec.Command(ctx, "echo", "test").
		WithBackground(true).
		Output()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunQuiet_Good(t *testing.T) {
	ctx := context.Background()
	err := exec.RunQuiet(ctx, "echo", "quiet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunQuiet_Bad(t *testing.T) {
	ctx := context.Background()
	err := exec.RunQuiet(ctx, "sh", "-c", "echo fail >&2; exit 1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExec_Command_Good(t *testing.T) {
	cmd := exec.Command(context.Background(), "echo", "hello")
	err := cmd.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExec_Command_Bad(t *testing.T) {
	cmd := exec.Command(nil, "echo")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected nil context error")
	}
}

func TestExec_Command_Ugly(t *testing.T) {
	cmd := exec.Command(context.Background(), "")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected empty command error")
	}
}

func TestExec_Cmd_WithDir_Good(t *testing.T) {
	out, err := exec.Command(context.Background(), "pwd").WithDir("/tmp").Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed != "/tmp" && trimmed != "/private/tmp" {
		t.Fatalf("unexpected pwd: %q", trimmed)
	}
}

func TestExec_Cmd_WithDir_Bad(t *testing.T) {
	_, err := exec.Command(context.Background(), "pwd").WithDir("/definitely/missing").Output()
	if err == nil {
		t.Fatal("expected missing directory error")
	}
}

func TestExec_Cmd_WithDir_Ugly(t *testing.T) {
	out, err := exec.Command(context.Background(), "pwd").WithDir("").Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(string(out)) == "" {
		t.Fatal("expected working directory output")
	}
}

func TestExec_Cmd_WithEnv_Good(t *testing.T) {
	out, err := exec.Command(context.Background(), "sh", "-c", "printf %s \"$AX7_ENV\"").WithEnv([]string{"AX7_ENV=ok"}).Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "ok" {
		t.Fatalf("expected env output, got %q", string(out))
	}
}

func TestExec_Cmd_WithEnv_Bad(t *testing.T) {
	out, err := exec.Command(context.Background(), "sh", "-c", "printf %s \"$AX7_ENV\"").WithEnv(nil).Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "" {
		t.Fatalf("expected empty env output, got %q", string(out))
	}
}

func TestExec_Cmd_WithEnv_Ugly(t *testing.T) {
	out, err := exec.Command(context.Background(), "sh", "-c", "printf %s \"$AX7_ENV\"").WithEnv([]string{"AX7_ENV="}).Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "" {
		t.Fatalf("expected empty env output, got %q", string(out))
	}
}

func TestExec_Cmd_WithStdin_Good(t *testing.T) {
	out, err := exec.Command(context.Background(), "cat").WithStdin(strings.NewReader("input")).Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "input" {
		t.Fatalf("expected stdin echoed, got %q", string(out))
	}
}

func TestExec_Cmd_WithStdin_Bad(t *testing.T) {
	out, err := exec.Command(context.Background(), "cat").WithStdin(nil).Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "" {
		t.Fatalf("expected empty output, got %q", string(out))
	}
}

func TestExec_Cmd_WithStdin_Ugly(t *testing.T) {
	out, err := exec.Command(context.Background(), "cat").WithStdin(strings.NewReader("")).Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "" {
		t.Fatalf("expected empty output, got %q", string(out))
	}
}

func TestExec_Cmd_WithStdout_Good(t *testing.T) {
	var stdout strings.Builder
	err := exec.Command(context.Background(), "echo", "hello").WithStdout(&stdout).Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "hello") {
		t.Fatalf("expected stdout capture, got %q", stdout.String())
	}
}

func TestExec_Cmd_WithStdout_Bad(t *testing.T) {
	var stdout strings.Builder
	err := exec.Command(context.Background(), "false").WithStdout(&stdout).Run()
	if err == nil {
		t.Fatal("expected command error")
	}
	if stdout.String() != "" {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
}

func TestExec_Cmd_WithStdout_Ugly(t *testing.T) {
	var stdout strings.Builder
	err := exec.Command(context.Background(), "printf", "").WithStdout(&stdout).Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.String() != "" {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
}

func TestExec_Cmd_WithStderr_Good(t *testing.T) {
	var stderr strings.Builder
	err := exec.Command(context.Background(), "sh", "-c", "echo err >&2").WithStderr(&stderr).Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "err") {
		t.Fatalf("expected stderr capture, got %q", stderr.String())
	}
}

func TestExec_Cmd_WithStderr_Bad(t *testing.T) {
	var stderr strings.Builder
	err := exec.Command(context.Background(), "sh", "-c", "echo err >&2; exit 2").WithStderr(&stderr).Run()
	if err == nil {
		t.Fatal("expected command error")
	}
	if !strings.Contains(stderr.String(), "err") {
		t.Fatalf("expected stderr capture, got %q", stderr.String())
	}
}

func TestExec_Cmd_WithStderr_Ugly(t *testing.T) {
	var stderr strings.Builder
	err := exec.Command(context.Background(), "printf", "").WithStderr(&stderr).Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stderr.String() != "" {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestExec_Cmd_WithLogger_Good(t *testing.T) {
	logger := &mockLogger{}
	err := exec.Command(context.Background(), "echo", "hello").WithLogger(logger).Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(logger.debugCalls) != 1 {
		t.Fatalf("expected debug log")
	}
}

func TestExec_Cmd_WithLogger_Bad(t *testing.T) {
	logger := &mockLogger{}
	err := exec.Command(context.Background(), "false").WithLogger(logger).Run()
	if err == nil {
		t.Fatal("expected error")
	}
	if len(logger.errorCalls) != 1 {
		t.Fatalf("expected error log")
	}
}

func TestExec_Cmd_WithLogger_Ugly(t *testing.T) {
	err := exec.Command(context.Background(), "echo", "hello").WithLogger(nil).Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExec_Cmd_WithBackground_Good(t *testing.T) {
	err := exec.Command(context.Background(), "true").WithBackground(true).Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExec_Cmd_WithBackground_Bad(t *testing.T) {
	_, err := exec.Command(context.Background(), "true").WithBackground(true).Output()
	if err == nil {
		t.Fatal("expected background output error")
	}
}

func TestExec_Cmd_WithBackground_Ugly(t *testing.T) {
	err := exec.Command(context.Background(), "false").WithBackground(true).Run()
	if err != nil {
		t.Fatalf("start should succeed in background: %v", err)
	}
}

func TestExec_Cmd_Start_Good(t *testing.T) {
	err := exec.Command(context.Background(), "true").WithBackground(true).Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExec_Cmd_Start_Bad(t *testing.T) {
	err := exec.Command(nil, "true").Start()
	if err == nil {
		t.Fatal("expected nil context error")
	}
}

func TestExec_Cmd_Start_Ugly(t *testing.T) {
	err := exec.Command(context.Background(), "definitely-not-a-real-command").Start()
	if err == nil {
		t.Fatal("expected start error")
	}
}

func TestExec_Cmd_Run_Good(t *testing.T) {
	err := exec.Command(context.Background(), "true").Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExec_Cmd_Run_Bad(t *testing.T) {
	err := exec.Command(context.Background(), "false").Run()
	if err == nil {
		t.Fatal("expected run error")
	}
}

func TestExec_Cmd_Run_Ugly(t *testing.T) {
	err := exec.Command(nil, "true").Run()
	if err == nil {
		t.Fatal("expected nil context error")
	}
}

func TestExec_Cmd_Output_Good(t *testing.T) {
	out, err := exec.Command(context.Background(), "echo", "hello").Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), "hello") {
		t.Fatalf("expected output, got %q", string(out))
	}
}

func TestExec_Cmd_Output_Bad(t *testing.T) {
	out, err := exec.Command(context.Background(), "false").Output()
	if err == nil {
		t.Fatal("expected output error")
	}
	if out != nil {
		t.Fatalf("expected nil output, got %q", string(out))
	}
}

func TestExec_Cmd_Output_Ugly(t *testing.T) {
	_, err := exec.Command(context.Background(), "echo").WithBackground(true).Output()
	if err == nil {
		t.Fatal("expected background output error")
	}
}

func TestExec_Cmd_CombinedOutput_Good(t *testing.T) {
	out, err := exec.Command(context.Background(), "sh", "-c", "echo out; echo err >&2").CombinedOutput()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), "out") || !strings.Contains(string(out), "err") {
		t.Fatalf("expected combined output, got %q", string(out))
	}
}

func TestExec_Cmd_CombinedOutput_Bad(t *testing.T) {
	out, err := exec.Command(context.Background(), "sh", "-c", "echo bad; exit 2").CombinedOutput()
	if err == nil {
		t.Fatal("expected combined output error")
	}
	if !strings.Contains(string(out), "bad") {
		t.Fatalf("expected partial output, got %q", string(out))
	}
}

func TestExec_Cmd_CombinedOutput_Ugly(t *testing.T) {
	_, err := exec.Command(context.Background(), "echo").WithBackground(true).CombinedOutput()
	if err == nil {
		t.Fatal("expected background combined output error")
	}
}

func TestExec_RunQuiet_Good(t *testing.T) {
	err := exec.RunQuiet(context.Background(), "true")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExec_RunQuiet_Bad(t *testing.T) {
	err := exec.RunQuiet(context.Background(), "false")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExec_RunQuiet_Ugly(t *testing.T) {
	err := exec.RunQuiet(nil, "true")
	if err == nil {
		t.Fatal("expected nil context error")
	}
}
