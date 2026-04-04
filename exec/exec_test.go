package exec_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"dappco.re/go/core/process/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	assert.NotNil(t, exec.DefaultLogger())
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
		require.Error(t, err)
		assert.ErrorIs(t, err, exec.ErrCommandContextRequired)
	})

	t.Run("run", func(t *testing.T) {
		err := exec.Command(nil, "echo", "test").Run()
		require.Error(t, err)
		assert.ErrorIs(t, err, exec.ErrCommandContextRequired)
	})

	t.Run("output", func(t *testing.T) {
		_, err := exec.Command(nil, "echo", "test").Output()
		require.Error(t, err)
		assert.ErrorIs(t, err, exec.ErrCommandContextRequired)
	})

	t.Run("combined output", func(t *testing.T) {
		_, err := exec.Command(nil, "echo", "test").CombinedOutput()
		require.Error(t, err)
		assert.ErrorIs(t, err, exec.ErrCommandContextRequired)
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
