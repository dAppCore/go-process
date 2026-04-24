package process

import coreerr "dappco.re/go/log"

// ServiceError wraps a service-level failure with a message string.
//
// Example:
//
//	return process.ServiceError("context is required", process.ErrContextRequired)
func ServiceError(message string, err error) error {
	return coreerr.E("ServiceError", message, err)
}
