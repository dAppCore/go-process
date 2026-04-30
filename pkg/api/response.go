// SPDX-Licence-Identifier: EUPL-1.2

package api

import core "dappco.re/go"

type apierr struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func (e *apierr) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// elementSpec describes a custom element for GUI rendering.
type elementSpec struct {
	Tag    string `json:"tag"`
	Source string `json:"source"`
}

func fail(code, message string) core.Result {
	return core.Fail(&apierr{Code: code, Message: message})
}

func failWithDetails(code, message string, details any) core.Result {
	return core.Fail(&apierr{Code: code, Message: message, Details: details})
}
