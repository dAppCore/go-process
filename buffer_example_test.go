package process_test

import (
	. "dappco.re/go"
	process "dappco.re/go/process"
)

func ExampleNewRingBuffer() {
	rb := process.NewRingBuffer(8)
	Println(rb.Cap())
	// Output: 8
}

func ExampleRingBuffer_Write() {
	rb := process.NewRingBuffer(8)
	result := rb.Write([]byte("hello"))
	Println(result.Value, rb.String())
	// Output: 5 hello
}

func ExampleRingBuffer_String() {
	rb := process.NewRingBuffer(5)
	if result := rb.Write([]byte("hello world")); !result.OK {
		Println(result.Error())
	}
	Println(rb.String())
	// Output: world
}

func ExampleRingBuffer_Bytes() {
	rb := process.NewRingBuffer(4)
	if result := rb.Write([]byte("data")); !result.OK {
		Println(result.Error())
	}
	Println(string(rb.Bytes()))
	// Output: data
}

func ExampleRingBuffer_Len() {
	rb := process.NewRingBuffer(8)
	if result := rb.Write([]byte("abc")); !result.OK {
		Println(result.Error())
	}
	Println(rb.Len())
	// Output: 3
}

func ExampleRingBuffer_Cap() {
	rb := process.NewRingBuffer(8)
	Println(rb.Cap())
	// Output: 8
}

func ExampleRingBuffer_Reset() {
	rb := process.NewRingBuffer(8)
	if result := rb.Write([]byte("abc")); !result.OK {
		Println(result.Error())
	}
	rb.Reset()
	Println(rb.Len())
	// Output: 0
}
