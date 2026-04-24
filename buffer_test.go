package process

import (
	"testing"
)

func TestRingBuffer_Basics_Good(t *testing.T) {
	t.Run("write and read", func(t *testing.T) {
		rb := NewRingBuffer(10)

		n, err := rb.Write([]byte("hello"))
		assertNoError(t, err)
		assertEqual(t, 5, n)
		assertEqual(t, "hello", rb.String())
		assertEqual(t, 5, rb.Len())
	})

	t.Run("overflow wraps around", func(t *testing.T) {
		rb := NewRingBuffer(5)

		_, _ = rb.Write([]byte("hello"))
		assertEqual(t, "hello", rb.String())

		_, _ = rb.Write([]byte("world"))
		// Should contain "world" (overwrote "hello")
		assertEqual(t, 5, rb.Len())
		assertEqual(t, "world", rb.String())
	})

	t.Run("partial overflow", func(t *testing.T) {
		rb := NewRingBuffer(10)

		_, _ = rb.Write([]byte("hello"))
		_, _ = rb.Write([]byte("worldx"))
		// Should contain "lloworldx" (11 chars, buffer is 10)
		assertEqual(t, 10, rb.Len())
	})

	t.Run("empty buffer", func(t *testing.T) {
		rb := NewRingBuffer(10)
		assertEqual(t, "", rb.String())
		assertEqual(t, 0, rb.Len())
		assertNil(t, rb.Bytes())
	})

	t.Run("reset", func(t *testing.T) {
		rb := NewRingBuffer(10)
		_, _ = rb.Write([]byte("hello"))
		rb.Reset()
		assertEqual(t, "", rb.String())
		assertEqual(t, 0, rb.Len())
	})

	t.Run("cap", func(t *testing.T) {
		rb := NewRingBuffer(42)
		assertEqual(t, 42, rb.Cap())
	})

	t.Run("bytes returns copy", func(t *testing.T) {
		rb := NewRingBuffer(10)
		_, _ = rb.Write([]byte("hello"))

		bytes := rb.Bytes()
		assertEqual(t, []byte("hello"), bytes)

		// Modifying returned bytes shouldn't affect buffer
		bytes[0] = 'x'
		assertEqual(t, "hello", rb.String())
	})

	t.Run("zero or negative capacity is a no-op", func(t *testing.T) {
		for _, size := range []int{0, -1} {
			rb := NewRingBuffer(size)

			n, err := rb.Write([]byte("discarded"))
			assertNoError(t, err)
			assertEqual(t, len("discarded"), n)
			assertEqual(t, 0, rb.Cap())
			assertEqual(t, 0, rb.Len())
			assertEqual(t, "", rb.String())
			assertNil(t, rb.Bytes())
		}
	})
}
