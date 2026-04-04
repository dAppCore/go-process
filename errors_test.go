package process

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceError(t *testing.T) {
	err := ServiceError("service failed", ErrContextRequired)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service failed")
	assert.ErrorIs(t, err, ErrContextRequired)
}
