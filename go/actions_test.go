package process

import (
	"testing"

	core "dappco.re/go"
)

func TestActions_TaskProcessWaitError_Error_Good(t *testing.T) {
	err := &TaskProcessWaitError{Err: core.NewError("wait failed")}
	got := err.Error()
	assertEqual(t, "wait failed", got)
}

func TestActions_TaskProcessWaitError_Error_Bad(t *testing.T) {
	err := &TaskProcessWaitError{}
	got := err.Error()
	assertEqual(t, "", got)
}

func TestActions_TaskProcessWaitError_Error_Ugly(t *testing.T) {
	var err *TaskProcessWaitError
	got := err.Error()
	assertEqual(t, "", got)
}

func TestActions_TaskProcessWaitError_Unwrap_Good(t *testing.T) {
	cause := core.NewError("wait failed")
	err := &TaskProcessWaitError{Err: cause}
	assertEqual(t, cause, err.Unwrap())
}

func TestActions_TaskProcessWaitError_Unwrap_Bad(t *testing.T) {
	err := &TaskProcessWaitError{}
	got := err.Unwrap()
	assertNil(t, got)
}

func TestActions_TaskProcessWaitError_Unwrap_Ugly(t *testing.T) {
	var err *TaskProcessWaitError
	got := err.Unwrap()
	assertNil(t, got)
}
