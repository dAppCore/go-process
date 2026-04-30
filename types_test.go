package process

import (
	"testing"
	"time"
)

func TestTypes_Status_Good(t *testing.T) {
	status := StatusRunning
	assertEqual(t, Status("running"), status)
	assertTrue(t, status == StatusRunning)
}

func TestTypes_Status_Bad(t *testing.T) {
	status := Status("unknown")
	assertFalse(t, status == StatusRunning)
	assertEqual(t, Status("unknown"), status)
}

func TestTypes_Status_Ugly(t *testing.T) {
	var status Status
	assertEqual(t, Status(""), status)
	assertFalse(t, status == StatusExited)
}

func TestTypes_Stream_Good(t *testing.T) {
	stream := StreamStdout
	assertEqual(t, Stream("stdout"), stream)
	assertFalse(t, stream == StreamStderr)
}

func TestTypes_Stream_Bad(t *testing.T) {
	stream := Stream("custom")
	assertEqual(t, Stream("custom"), stream)
	assertFalse(t, stream == StreamStdout)
}

func TestTypes_Stream_Ugly(t *testing.T) {
	var stream Stream
	assertEqual(t, Stream(""), stream)
	assertFalse(t, stream == StreamStderr)
}

func TestTypes_RunOptions_Good(t *testing.T) {
	opts := RunOptions{Command: "echo", Args: []string{"ok"}, Timeout: time.Second}
	assertEqual(t, "echo", opts.Command)
	assertEqual(t, []string{"ok"}, opts.Args)
}

func TestTypes_RunOptions_Bad(t *testing.T) {
	opts := RunOptions{}
	assertEqual(t, "", opts.Command)
	assertFalse(t, opts.KillGroup)
}

func TestTypes_RunOptions_Ugly(t *testing.T) {
	opts := RunOptions{Detach: true, KillGroup: true, GracePeriod: time.Nanosecond}
	assertTrue(t, opts.Detach)
	assertEqual(t, time.Nanosecond, opts.GracePeriod)
}

func TestTypes_Info_Good(t *testing.T) {
	info := Info{ID: "proc-1", Status: StatusExited, ExitCode: 0}
	assertEqual(t, "proc-1", info.ID)
	assertEqual(t, StatusExited, info.Status)
}

func TestTypes_Info_Bad(t *testing.T) {
	info := Info{Status: StatusFailed, ExitCode: -1}
	assertEqual(t, StatusFailed, info.Status)
	assertEqual(t, -1, info.ExitCode)
}

func TestTypes_Info_Ugly(t *testing.T) {
	var info Info
	assertEqual(t, "", info.ID)
	assertFalse(t, info.Running)
}
