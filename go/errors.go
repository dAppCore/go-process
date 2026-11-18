package process

import (
	core "github.com/dappcore/go"
)

// ServiceError wraps a service-level failure with a message string.
//
// Example:
//
//	return process.ServiceError("context is required", process.ErrContextRequired)
func ServiceError(message string, err error) core.Result {
	return core.Fail(core.E("ServiceError", message, err))
}
