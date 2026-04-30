package command_test

import (
	"testing"

	command "dappco.re/go/process/exec"
)

func TestLogger_NopLogger_Debug_Good(t *testing.T) {
	var logger command.NopLogger
	logger.Debug("debug", "key", "value")
	if command.DefaultLogger() == nil {
		t.Fatal("expected default logger")
	}
}

func TestLogger_NopLogger_Debug_Bad(t *testing.T) {
	var logger command.NopLogger
	logger.Debug("")
	if command.DefaultLogger() == nil {
		t.Fatal("expected default logger")
	}
}

func TestLogger_NopLogger_Debug_Ugly(t *testing.T) {
	var logger command.NopLogger
	logger.Debug("debug", []any{}...)
	if command.DefaultLogger() == nil {
		t.Fatal("expected default logger")
	}
}

func TestLogger_NopLogger_Error_Good(t *testing.T) {
	var logger command.NopLogger
	logger.Error("error", "key", "value")
	if command.DefaultLogger() == nil {
		t.Fatal("expected default logger")
	}
}

func TestLogger_NopLogger_Error_Bad(t *testing.T) {
	var logger command.NopLogger
	logger.Error("")
	if command.DefaultLogger() == nil {
		t.Fatal("expected default logger")
	}
}

func TestLogger_NopLogger_Error_Ugly(t *testing.T) {
	var logger command.NopLogger
	logger.Error("error", []any{}...)
	if command.DefaultLogger() == nil {
		t.Fatal("expected default logger")
	}
}

func TestLogger_SetDefaultLogger_Good(t *testing.T) {
	original := command.DefaultLogger()
	t.Cleanup(func() { command.SetDefaultLogger(original) })
	logger := &mockLogger{}
	command.SetDefaultLogger(logger)
	if command.DefaultLogger() != logger {
		t.Fatal("expected custom default logger")
	}
}

func TestLogger_SetDefaultLogger_Bad(t *testing.T) {
	original := command.DefaultLogger()
	t.Cleanup(func() { command.SetDefaultLogger(original) })
	command.SetDefaultLogger(nil)
	if _, ok := command.DefaultLogger().(command.NopLogger); !ok {
		t.Fatal("expected nil to reset to NopLogger")
	}
}

func TestLogger_SetDefaultLogger_Ugly(t *testing.T) {
	original := command.DefaultLogger()
	t.Cleanup(func() { command.SetDefaultLogger(original) })
	first := &mockLogger{}
	second := &mockLogger{}
	command.SetDefaultLogger(first)
	command.SetDefaultLogger(second)
	if command.DefaultLogger() != second {
		t.Fatal("expected latest logger")
	}
}

func TestLogger_DefaultLogger_Good(t *testing.T) {
	logger := command.DefaultLogger()
	if logger == nil {
		t.Fatal("expected logger")
	}
	logger.Debug("debug")
}

func TestLogger_DefaultLogger_Bad(t *testing.T) {
	original := command.DefaultLogger()
	t.Cleanup(func() { command.SetDefaultLogger(original) })
	command.SetDefaultLogger(nil)
	logger := command.DefaultLogger()
	if _, ok := logger.(command.NopLogger); !ok {
		t.Fatal("expected NopLogger")
	}
}

func TestLogger_DefaultLogger_Ugly(t *testing.T) {
	original := command.DefaultLogger()
	t.Cleanup(func() { command.SetDefaultLogger(original) })
	logger := &mockLogger{}
	command.SetDefaultLogger(logger)
	got := command.DefaultLogger()
	if got != logger {
		t.Fatal("expected custom logger")
	}
}
