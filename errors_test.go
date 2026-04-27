package process

import (
	"testing"
)

func TestServiceError(t *testing.T) {
	err := ServiceError("service failed", ErrContextRequired)
	requireError(t, err)
	assertContains(t, err.Error(), "service failed")
	assertErrorIs(t, err, ErrContextRequired)
}
