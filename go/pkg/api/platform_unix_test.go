// SPDX-Licence-Identifier: EUPL-1.2

//go:build !windows

package api

import (
	"context"
	"syscall"
	"testing"

	process "dappco.re/go/process"
)

func TestPlatformUnix_signalPID_Good(t *testing.T) {
	svc := newTestProcessService(t)
	proc, err := resultValue[*process.Process](svc.Start(context.Background(), "sleep", "30"))
	requireNoError(t, err)
	t.Cleanup(func() {
		if proc.IsRunning() {
			_ = svc.Kill(proc.ID)
		}
	})

	// Signal 0 probes liveness without delivering a signal.
	r := signalPID(proc.Info().PID, syscall.Signal(0))
	assertTrue(t, r.OK)
}

func TestPlatformUnix_signalPID_Bad(t *testing.T) {
	// A dead PID yields a failed result (ESRCH).
	r := signalPID(2147483646, syscall.Signal(0))
	assertFalse(t, r.OK)
}

func TestPlatformUnix_pidAlive_Good(t *testing.T) {
	svc := newTestProcessService(t)
	proc, err := resultValue[*process.Process](svc.Start(context.Background(), "sleep", "30"))
	requireNoError(t, err)
	t.Cleanup(func() {
		if proc.IsRunning() {
			_ = svc.Kill(proc.ID)
		}
	})

	assertTrue(t, pidAlive(proc.Info().PID))
}

func TestPlatformUnix_pidAlive_Bad(t *testing.T) {
	assertFalse(t, pidAlive(2147483646))
}

func TestPlatformUnix_parsePlatformSignal_Good(t *testing.T) {
	cases := map[string]syscall.Signal{
		"SIGSTOP": syscall.SIGSTOP,
		"STOP":    syscall.SIGSTOP,
		"SIGCONT": syscall.SIGCONT,
		"CONT":    syscall.SIGCONT,
		"SIGUSR1": syscall.SIGUSR1,
		"USR1":    syscall.SIGUSR1,
		"SIGUSR2": syscall.SIGUSR2,
		"USR2":    syscall.SIGUSR2,
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			r := parsePlatformSignal(name)
			requireTrue(t, r.OK)
			assertEqual(t, want, r.Value.(syscall.Signal))
		})
	}
}

func TestPlatformUnix_parsePlatformSignal_Bad(t *testing.T) {
	r := parsePlatformSignal("SIGNOPE")
	assertFalse(t, r.OK)
}

func TestPlatformUnix_parsePlatformSignal_Ugly(t *testing.T) {
	// Empty string is unsupported.
	r := parsePlatformSignal("")
	assertFalse(t, r.OK)
}
