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
	n, _ := rb.Write([]byte("hello"))
	Println(n, rb.String())
	// Output: 5 hello
}

func ExampleRingBuffer_String() {
	rb := process.NewRingBuffer(5)
	rb.Write([]byte("hello world"))
	Println(rb.String())
	// Output: world
}

func ExampleRingBuffer_Bytes() {
	rb := process.NewRingBuffer(4)
	rb.Write([]byte("data"))
	Println(string(rb.Bytes()))
	// Output: data
}

func ExampleRingBuffer_Len() {
	rb := process.NewRingBuffer(8)
	rb.Write([]byte("abc"))
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
	rb.Write([]byte("abc"))
	rb.Reset()
	Println(rb.Len())
	// Output: 0
}
