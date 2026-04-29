package process

import (
	"testing"

	core "dappco.re/go"
)

func TestServiceError(t *testing.T) {
	result := ServiceError("service failed", ErrContextRequired)
	err := result.Value.(error)
	requireError(t, result)
	assertContains(t, err.Error(), "service failed")
	assertErrorIs(t, err, ErrContextRequired)
}

func TestErrors_ServiceError_Good(t *testing.T) {
	result := ServiceError("service failed", ErrContextRequired)
	err := result.Value.(error)
	requireError(t, result)
	assertContains(t, err.Error(), "service failed")
	assertErrorIs(t, err, ErrContextRequired)
}

func TestErrors_ServiceError_Bad(t *testing.T) {
	result := ServiceError("service failed", nil)
	err := result.Value.(error)
	requireError(t, result)
	assertContains(t, err.Error(), "service failed")
	assertFalse(t, core.Is(err, ErrContextRequired))
}

func TestErrors_ServiceError_Ugly(t *testing.T) {
	result := ServiceError("", nil)
	err := result.Value.(error)
	requireError(t, result)
	assertContains(t, err.Error(), "ServiceError")
	unwrapper, ok := err.(interface{ Unwrap() error })
	requireTrue(t, ok)
	assertNil(t, unwrapper.Unwrap())
}
