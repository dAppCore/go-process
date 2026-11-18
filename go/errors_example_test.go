package process_test

import (
	. "github.com/dappcore/go"
	process "github.com/dappcore/go-process/go"
)

func ExampleServiceError() {
	result := process.ServiceError("start failed", process.ErrContextRequired)
	Println(result.OK, Contains(result.Error(), "start failed"))
	// Output: false true
}
