package process

import (
	core "dappco.re/go"
)

// ServiceError wraps a service-level failure with a message string.
//
// Example:
//
//	return process.ServiceError("context is required", process.ErrContextRequired)
func ServiceError(message string, err error) core.Result {
	return core.Fail(core.E("ServiceError", message, err))
}
