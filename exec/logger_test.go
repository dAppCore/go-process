package command_test

import (
	"testing"

	exec "dappco.re/go/process/exec"
)

func TestLogger_NopLogger_Debug_Good(t *testing.T) {
	var logger exec.NopLogger
	logger.Debug("debug", "key", "value")
	if exec.DefaultLogger() == nil {
		t.Fatal("expected default logger")
	}
}

func TestLogger_NopLogger_Debug_Bad(t *testing.T) {
	var logger exec.NopLogger
	logger.Debug("")
	if exec.DefaultLogger() == nil {
		t.Fatal("expected default logger")
	}
}

func TestLogger_NopLogger_Debug_Ugly(t *testing.T) {
	var logger exec.NopLogger
	logger.Debug("debug", []any{}...)
	if exec.DefaultLogger() == nil {
		t.Fatal("expected default logger")
	}
}

func TestLogger_NopLogger_Error_Good(t *testing.T) {
	var logger exec.NopLogger
	logger.Error("error", "key", "value")
	if exec.DefaultLogger() == nil {
		t.Fatal("expected default logger")
	}
}

func TestLogger_NopLogger_Error_Bad(t *testing.T) {
	var logger exec.NopLogger
	logger.Error("")
	if exec.DefaultLogger() == nil {
		t.Fatal("expected default logger")
	}
}

func TestLogger_NopLogger_Error_Ugly(t *testing.T) {
	var logger exec.NopLogger
	logger.Error("error", []any{}...)
	if exec.DefaultLogger() == nil {
		t.Fatal("expected default logger")
	}
}

func TestLogger_SetDefaultLogger_Good(t *testing.T) {
	original := exec.DefaultLogger()
	t.Cleanup(func() { exec.SetDefaultLogger(original) })
	logger := &mockLogger{}
	exec.SetDefaultLogger(logger)
	if exec.DefaultLogger() != logger {
		t.Fatal("expected custom default logger")
	}
}

func TestLogger_SetDefaultLogger_Bad(t *testing.T) {
	original := exec.DefaultLogger()
	t.Cleanup(func() { exec.SetDefaultLogger(original) })
	exec.SetDefaultLogger(nil)
	if _, ok := exec.DefaultLogger().(exec.NopLogger); !ok {
		t.Fatal("expected nil to reset to NopLogger")
	}
}

func TestLogger_SetDefaultLogger_Ugly(t *testing.T) {
	original := exec.DefaultLogger()
	t.Cleanup(func() { exec.SetDefaultLogger(original) })
	first := &mockLogger{}
	second := &mockLogger{}
	exec.SetDefaultLogger(first)
	exec.SetDefaultLogger(second)
	if exec.DefaultLogger() != second {
		t.Fatal("expected latest logger")
	}
}

func TestLogger_DefaultLogger_Good(t *testing.T) {
	logger := exec.DefaultLogger()
	if logger == nil {
		t.Fatal("expected logger")
	}
	logger.Debug("debug")
}

func TestLogger_DefaultLogger_Bad(t *testing.T) {
	original := exec.DefaultLogger()
	t.Cleanup(func() { exec.SetDefaultLogger(original) })
	exec.SetDefaultLogger(nil)
	logger := exec.DefaultLogger()
	if _, ok := logger.(exec.NopLogger); !ok {
		t.Fatal("expected NopLogger")
	}
}

func TestLogger_DefaultLogger_Ugly(t *testing.T) {
	original := exec.DefaultLogger()
	t.Cleanup(func() { exec.SetDefaultLogger(original) })
	logger := &mockLogger{}
	exec.SetDefaultLogger(logger)
	got := exec.DefaultLogger()
	if got != logger {
		t.Fatal("expected custom logger")
	}
}
