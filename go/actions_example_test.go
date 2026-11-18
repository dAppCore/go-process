package process_test

import (
	. "github.com/dappcore/go"
	process "github.com/dappcore/go-process/go"
)

func ExampleTaskProcessWaitError_Error() {
	err := &process.TaskProcessWaitError{Err: NewError("wait failed")}
	Println(err.Error())
	// Output: wait failed
}

func ExampleTaskProcessWaitError_Unwrap() {
	cause := NewError("wait failed")
	err := &process.TaskProcessWaitError{Err: cause}
	Println(Is(err.Unwrap(), cause))
	// Output: true
}
