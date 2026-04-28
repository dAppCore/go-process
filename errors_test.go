package process

import (
	"errors"
	"testing"
)

func TestServiceError(t *testing.T) {
	err := ServiceError("service failed", ErrContextRequired)
	requireError(t, err)
	assertContains(t, err.Error(), "service failed")
	assertErrorIs(t, err, ErrContextRequired)
}

func TestErrors_ServiceError_Good(t *testing.T) {
	err := ServiceError("service failed", ErrContextRequired)
	requireError(t, err)
	assertContains(t, err.Error(), "service failed")
	assertErrorIs(t, err, ErrContextRequired)
}

func TestErrors_ServiceError_Bad(t *testing.T) {
	err := ServiceError("service failed", nil)
	requireError(t, err)
	assertContains(t, err.Error(), "service failed")
	assertFalse(t, errors.Is(err, ErrContextRequired))
}

func TestErrors_ServiceError_Ugly(t *testing.T) {
	err := ServiceError("", nil)
	requireError(t, err)
	assertContains(t, err.Error(), "ServiceError")
	assertNil(t, errors.Unwrap(err))
}
