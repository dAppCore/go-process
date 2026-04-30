package process_test

import (
	. "dappco.re/go"
	process "dappco.re/go/process"
)

func ExampleServiceError() {
	result := process.ServiceError("start failed", process.ErrContextRequired)
	Println(result.OK, Contains(result.Error(), "start failed"))
	// Output: false true
}
