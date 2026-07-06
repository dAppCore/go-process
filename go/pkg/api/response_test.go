// SPDX-Licence-Identifier: EUPL-1.2

package api

import "testing"

func TestResponse_apierr_Error_Good(t *testing.T) {
	e := &apierr{Code: "bad_request", Message: "invalid payload"}
	assertEqual(t, "invalid payload", e.Error())
}

func TestResponse_apierr_Error_Bad(t *testing.T) {
	// Empty message yields an empty string.
	e := &apierr{Code: "bad_request"}
	assertEqual(t, "", e.Error())
}

func TestResponse_apierr_Error_Ugly(t *testing.T) {
	// A nil receiver must not panic.
	var e *apierr
	assertEqual(t, "", e.Error())
}

func TestResponse_fail_Good(t *testing.T) {
	r := fail("not_found", "missing")
	assertFalse(t, r.OK)
	err, ok := r.Value.(*apierr)
	requireTrue(t, ok)
	assertEqual(t, "not_found", err.Code)
	assertEqual(t, "missing", err.Message)
	assertNil(t, err.Details)
}

func TestResponse_failWithDetails_Good(t *testing.T) {
	r := failWithDetails("conflict", "already exists", map[string]string{"id": "x"})
	assertFalse(t, r.OK)
	err, ok := r.Value.(*apierr)
	requireTrue(t, ok)
	assertEqual(t, "conflict", err.Code)
	assertNotNil(t, err.Details)
}
