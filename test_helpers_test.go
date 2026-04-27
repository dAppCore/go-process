package process

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	core "dappco.re/go/core"
)

var errSentinel = core.E("", "sentinel error", nil)

func assertEqual(t *testing.T, want, got any, msgAndArgs ...any) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		t.Errorf("want %v (%T), got %v (%T)%s", want, want, got, got, helperMessage(msgAndArgs...))
	}
}

func assertTrue(t *testing.T, value bool, msgAndArgs ...any) {
	t.Helper()
	if !value {
		t.Errorf("expected true%s", helperMessage(msgAndArgs...))
	}
}

func requireTrue(t *testing.T, value bool, msgAndArgs ...any) {
	t.Helper()
	if !value {
		t.Fatalf("expected true%s", helperMessage(msgAndArgs...))
	}
}

func assertFalse(t *testing.T, value bool, msgAndArgs ...any) {
	t.Helper()
	if value {
		t.Errorf("expected false%s", helperMessage(msgAndArgs...))
	}
}

func requireFalse(t *testing.T, value bool, msgAndArgs ...any) {
	t.Helper()
	if value {
		t.Fatalf("expected false%s", helperMessage(msgAndArgs...))
	}
}

func assertNil(t *testing.T, value any, msgAndArgs ...any) {
	t.Helper()
	if !isNil(value) {
		t.Errorf("expected nil, got %v%s", value, helperMessage(msgAndArgs...))
	}
}

func assertNotNil(t *testing.T, value any, msgAndArgs ...any) {
	t.Helper()
	if isNil(value) {
		t.Errorf("expected non-nil%s", helperMessage(msgAndArgs...))
	}
}

func requireNotNil(t *testing.T, value any, msgAndArgs ...any) {
	t.Helper()
	if isNil(value) {
		t.Fatalf("expected non-nil%s", helperMessage(msgAndArgs...))
	}
}

func assertNoError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()
	if err != nil {
		t.Errorf("unexpected error: %v%s", err, helperMessage(msgAndArgs...))
	}
}

func requireNoError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v%s", err, helperMessage(msgAndArgs...))
	}
}

func assertError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()
	if err == nil {
		t.Errorf("expected error%s", helperMessage(msgAndArgs...))
	}
}

func requireError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error%s", helperMessage(msgAndArgs...))
	}
}

func assertErrorIs(t *testing.T, err, target error, msgAndArgs ...any) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Errorf("expected error %v to match %v%s", err, target, helperMessage(msgAndArgs...))
	}
}

func requireErrorIs(t *testing.T, err, target error, msgAndArgs ...any) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("expected error %v to match %v%s", err, target, helperMessage(msgAndArgs...))
	}
}

func requireErrorAs(t *testing.T, err error, target any, msgAndArgs ...any) {
	t.Helper()
	if !errors.As(err, target) {
		t.Fatalf("expected error %v to assign to %T%s", err, target, helperMessage(msgAndArgs...))
	}
}

func assertContains(t *testing.T, container, item any, msgAndArgs ...any) {
	t.Helper()
	if !containsValue(container, item) {
		t.Errorf("expected %v to contain %v%s", container, item, helperMessage(msgAndArgs...))
	}
}

func assertEmpty(t *testing.T, value any, msgAndArgs ...any) {
	t.Helper()
	if !isEmpty(value) {
		t.Errorf("expected empty, got %v%s", value, helperMessage(msgAndArgs...))
	}
}

func assertNotEmpty(t *testing.T, value any, msgAndArgs ...any) {
	t.Helper()
	if isEmpty(value) {
		t.Errorf("expected non-empty%s", helperMessage(msgAndArgs...))
	}
}

func requireNotEmpty(t *testing.T, value any, msgAndArgs ...any) {
	t.Helper()
	if isEmpty(value) {
		t.Fatalf("expected non-empty%s", helperMessage(msgAndArgs...))
	}
}

func assertLen(t *testing.T, value any, want int, msgAndArgs ...any) {
	t.Helper()
	got, ok := valueLen(value)
	if !ok || got != want {
		t.Errorf("expected length %d, got %d%s", want, got, helperMessage(msgAndArgs...))
	}
}

func requireLen(t *testing.T, value any, want int, msgAndArgs ...any) {
	t.Helper()
	got, ok := valueLen(value)
	if !ok || got != want {
		t.Fatalf("expected length %d, got %d%s", want, got, helperMessage(msgAndArgs...))
	}
}

func assertGreater(t *testing.T, got, want any, msgAndArgs ...any) {
	t.Helper()
	cmp, ok := compareNumbers(got, want)
	if !ok || cmp <= 0 {
		t.Errorf("expected %v to be greater than %v%s", got, want, helperMessage(msgAndArgs...))
	}
}

func assertGreaterOrEqual(t *testing.T, got, want any, msgAndArgs ...any) {
	t.Helper()
	cmp, ok := compareNumbers(got, want)
	if !ok || cmp < 0 {
		t.Errorf("expected %v to be greater than or equal to %v%s", got, want, helperMessage(msgAndArgs...))
	}
}

func assertLess(t *testing.T, got, want any, msgAndArgs ...any) {
	t.Helper()
	cmp, ok := compareNumbers(got, want)
	if !ok || cmp >= 0 {
		t.Errorf("expected %v to be less than %v%s", got, want, helperMessage(msgAndArgs...))
	}
}

func requireEventually(t *testing.T, condition func() bool, timeout, interval time.Duration, msgAndArgs ...any) {
	t.Helper()
	if interval <= 0 {
		interval = 10 * time.Millisecond
	}
	deadline := time.Now().Add(timeout)
	for {
		if condition() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("condition was not met within %s%s", timeout, helperMessage(msgAndArgs...))
		}
		time.Sleep(interval)
	}
}

func helperMessage(args ...any) string {
	if len(args) == 0 {
		return ""
	}
	format, ok := args[0].(string)
	if !ok {
		return ": " + fmt.Sprint(args...)
	}
	if len(args) == 1 {
		return ": " + format
	}
	return ": " + fmt.Sprintf(format, args[1:]...)
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

func isEmpty(value any) bool {
	if isNil(value) {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	default:
		return reflect.DeepEqual(value, reflect.Zero(v.Type()).Interface())
	}
}

func valueLen(value any) (int, bool) {
	if value == nil {
		return 0, true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
		return v.Len(), true
	default:
		return 0, false
	}
}

func containsValue(container, item any) bool {
	if s, ok := container.(string); ok {
		sub, ok := item.(string)
		return ok && strings.Contains(s, sub)
	}
	if isNil(container) {
		return false
	}
	v := reflect.ValueOf(container)
	switch v.Kind() {
	case reflect.Array, reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if reflect.DeepEqual(v.Index(i).Interface(), item) {
				return true
			}
		}
	case reflect.Map:
		key := reflect.ValueOf(item)
		if !key.IsValid() {
			return false
		}
		if key.Type().AssignableTo(v.Type().Key()) {
			return v.MapIndex(key).IsValid()
		}
		if key.Type().ConvertibleTo(v.Type().Key()) {
			return v.MapIndex(key.Convert(v.Type().Key())).IsValid()
		}
	}
	return false
}

func compareNumbers(got, want any) (int, bool) {
	left, ok := numberAsFloat(got)
	if !ok {
		return 0, false
	}
	right, ok := numberAsFloat(want)
	if !ok {
		return 0, false
	}
	switch {
	case left < right:
		return -1, true
	case left > right:
		return 1, true
	default:
		return 0, true
	}
}

func numberAsFloat(value any) (float64, bool) {
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return float64(v.Uint()), true
	case reflect.Float32, reflect.Float64:
		return v.Float(), true
	default:
		return 0, false
	}
}
