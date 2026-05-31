package process

import (
	"testing"
	"time"

	core "dappco.re/go"
)

// opts builds a core.Options from alternating key/value pairs for tests.
//
//	o := opts("timeout", time.Second, "pid", 42)
func opts(pairs ...any) core.Options {
	items := make([]core.Option, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		key, _ := pairs[i].(string)
		items = append(items, core.Option{Key: key, Value: pairs[i+1]})
	}
	return core.NewOptions(items...)
}

func TestActionsParse_parseDurationOption_Good(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  time.Duration
	}{
		{"duration", 2 * time.Second, 2 * time.Second},
		{"int", int(5), 5},
		{"int8", int8(5), 5},
		{"int16", int16(5), 5},
		{"int32", int32(5), 5},
		{"int64", int64(5), 5},
		{"uint", uint(5), 5},
		{"uint8", uint8(5), 5},
		{"uint16", uint16(5), 5},
		{"uint32", uint32(5), 5},
		{"uint64", uint64(5), 5},
		{"float32", float32(5), 5},
		{"float64", float64(5), 5},
		{"string-duration", "1500ms", 1500 * time.Millisecond},
		{"string-int", "750", time.Duration(750)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseDurationOption(opts("timeout", tc.value), "timeout")
			assertEqual(t, tc.want, got)
		})
	}
}

func TestActionsParse_parseDurationOption_Bad(t *testing.T) {
	// Missing key returns zero.
	assertEqual(t, time.Duration(0), parseDurationOption(opts(), "timeout"))
	// Unparseable string returns zero.
	assertEqual(t, time.Duration(0), parseDurationOption(opts("timeout", "not-a-duration"), "timeout"))
}

func TestActionsParse_parseDurationOption_Ugly(t *testing.T) {
	// Unsupported value type falls through to zero.
	assertEqual(t, time.Duration(0), parseDurationOption(opts("timeout", struct{}{}), "timeout"))
	assertEqual(t, time.Duration(0), parseDurationOption(opts("timeout", []byte("x")), "timeout"))
}

func TestActionsParse_parseIntOption_Good(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  int
	}{
		{"int", int(7), 7},
		{"int8", int8(7), 7},
		{"int16", int16(7), 7},
		{"int32", int32(7), 7},
		{"int64", int64(7), 7},
		{"uint", uint(7), 7},
		{"uint8", uint8(7), 7},
		{"uint16", uint16(7), 7},
		{"uint32", uint32(7), 7},
		{"uint64", uint64(7), 7},
		{"float32", float32(7), 7},
		{"float64", float64(7), 7},
		{"string", "42", 42},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseIntOption(opts("pid", tc.value), "pid")
			assertEqual(t, tc.want, got)
		})
	}
}

func TestActionsParse_parseIntOption_Bad(t *testing.T) {
	// Missing key returns zero.
	assertEqual(t, 0, parseIntOption(opts(), "pid"))
	// Non-numeric string returns zero.
	assertEqual(t, 0, parseIntOption(opts("pid", "abc"), "pid"))
}

func TestActionsParse_parseIntOption_Ugly(t *testing.T) {
	// Unsupported type returns zero.
	assertEqual(t, 0, parseIntOption(opts("pid", struct{}{}), "pid"))
	assertEqual(t, 0, parseIntOption(opts("pid", []int{1}), "pid"))
}

func TestActionsParse_parseStringSliceOption_Good(t *testing.T) {
	// Native []string passes through unchanged.
	r := parseStringSliceOption(opts("args", []string{"a", "b"}), "args")
	requireTrue(t, r.OK)
	assertEqual(t, []string{"a", "b"}, r.Value.([]string))

	// []any of strings is converted.
	r = parseStringSliceOption(opts("args", []any{"x", "y"}), "args")
	requireTrue(t, r.OK)
	assertEqual(t, []string{"x", "y"}, r.Value.([]string))

	// []interface{} alias of strings is converted.
	r = parseStringSliceOption(opts("args", []interface{}{"p"}), "args")
	requireTrue(t, r.OK)
	assertEqual(t, []string{"p"}, r.Value.([]string))
}

func TestActionsParse_parseStringSliceOption_Bad(t *testing.T) {
	// Missing key returns an OK nil slice (no args supplied).
	r := parseStringSliceOption(opts(), "args")
	requireTrue(t, r.OK)
	assertNil(t, r.Value.([]string))

	// Non-array value is rejected.
	r = parseStringSliceOption(opts("args", "single"), "args")
	assertFalse(t, r.OK)
}

func TestActionsParse_parseStringSliceOption_Ugly(t *testing.T) {
	// A []any containing a non-string entry is rejected.
	r := parseStringSliceOption(opts("args", []any{"ok", 7}), "args")
	assertFalse(t, r.OK)
}

func TestActionsParse_parseProcessActionTarget_Good(t *testing.T) {
	// Target resolves by ID.
	r := parseProcessActionTarget(opts("id", "proc-1"))
	requireTrue(t, r.OK)
	in := r.Value.(processActionInput)
	assertEqual(t, "proc-1", in.ID)

	// Target resolves by PID.
	r = parseProcessActionTarget(opts("pid", 1234))
	requireTrue(t, r.OK)
	in = r.Value.(processActionInput)
	assertEqual(t, 1234, in.PID)
}

func TestActionsParse_parseProcessActionTarget_Bad(t *testing.T) {
	// Neither id nor pid is an error.
	r := parseProcessActionTarget(opts())
	assertFalse(t, r.OK)

	// Whitespace-only id with no pid is also an error (trimmed empty).
	r = parseProcessActionTarget(opts("id", "   "))
	assertFalse(t, r.OK)
}

func TestActionsParse_parseProcessActionTarget_Ugly(t *testing.T) {
	// Non-positive PID with no id is rejected.
	r := parseProcessActionTarget(opts("pid", 0))
	assertFalse(t, r.OK)
	r = parseProcessActionTarget(opts("pid", -5))
	assertFalse(t, r.OK)
}

func TestActionsParse_parseProcessActionInput_Good(t *testing.T) {
	r := parseProcessActionInput(opts(
		"command", "  echo  ",
		"args", []string{"hello"},
		"dir", "/tmp",
		"env", []string{"A=1"},
		"disableCapture", true,
		"detach", true,
		"killGroup", true,
		"timeout", 3*time.Second,
		"gracePeriod", time.Second,
		"pid", 99,
		"id", "proc-x",
	), true)
	requireTrue(t, r.OK)
	in := r.Value.(processActionInput)
	assertEqual(t, "echo", in.Command)
	assertEqual(t, []string{"hello"}, in.Args)
	assertEqual(t, "/tmp", in.Dir)
	assertEqual(t, []string{"A=1"}, in.Env)
	assertTrue(t, in.DisableCapture)
	assertTrue(t, in.Detach)
	assertTrue(t, in.KillGroup)
	assertEqual(t, 3*time.Second, in.Timeout)
	assertEqual(t, time.Second, in.GracePeriod)
	assertEqual(t, 99, in.PID)
	assertEqual(t, "proc-x", in.ID)
}

func TestActionsParse_parseProcessActionInput_Bad(t *testing.T) {
	// requireCommand with empty command is an error.
	r := parseProcessActionInput(opts(), true)
	assertFalse(t, r.OK)

	// requireCommand=false tolerates the missing command.
	r = parseProcessActionInput(opts(), false)
	assertTrue(t, r.OK)
}

func TestActionsParse_parseProcessActionInput_Ugly(t *testing.T) {
	// A malformed args slice propagates the parse failure.
	r := parseProcessActionInput(opts("command", "echo", "args", "not-a-slice"), true)
	assertFalse(t, r.OK)

	// A malformed env slice propagates the parse failure.
	r = parseProcessActionInput(opts("command", "echo", "env", []any{1}), true)
	assertFalse(t, r.OK)
}
