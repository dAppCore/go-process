package process

import (
	"testing"
)

func TestRingBufferBasics(t *testing.T) {
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

func TestBuffer_NewRingBuffer_Good(t *testing.T) {
	rb := NewRingBuffer(4)
	n, err := rb.Write([]byte("go"))
	requireNoError(t, err)
	assertEqual(t, 2, n)
	assertEqual(t, 4, rb.Cap())
}

func TestBuffer_NewRingBuffer_Bad(t *testing.T) {
	rb := NewRingBuffer(-1)
	n, err := rb.Write([]byte("drop"))
	requireNoError(t, err)
	assertEqual(t, 0, rb.Cap())
	assertEqual(t, 0, rb.Len())
	assertEqual(t, len("drop"), n)
}

func TestBuffer_NewRingBuffer_Ugly(t *testing.T) {
	rb := NewRingBuffer(0)
	n, err := rb.Write(nil)
	requireNoError(t, err)
	assertEqual(t, 0, n)
	assertEqual(t, "", rb.String())
}

func TestBuffer_RingBuffer_Write_Good(t *testing.T) {
	rb := NewRingBuffer(8)
	n, err := rb.Write([]byte("abc"))
	requireNoError(t, err)
	assertEqual(t, 3, n)
	assertEqual(t, "abc", rb.String())
}

func TestBuffer_RingBuffer_Write_Bad(t *testing.T) {
	rb := NewRingBuffer(0)
	n, err := rb.Write([]byte("abc"))
	requireNoError(t, err)
	assertEqual(t, len("abc"), n)
	assertEqual(t, "", rb.String())
}

func TestBuffer_RingBuffer_Write_Ugly(t *testing.T) {
	rb := NewRingBuffer(3)
	n, err := rb.Write([]byte("abcdef"))
	requireNoError(t, err)
	assertEqual(t, 6, n)
	assertEqual(t, "def", rb.String())
}

func TestBuffer_RingBuffer_String_Good(t *testing.T) {
	rb := NewRingBuffer(5)
	_, err := rb.Write([]byte("hello"))
	requireNoError(t, err)
	assertEqual(t, "hello", rb.String())
}

func TestBuffer_RingBuffer_String_Bad(t *testing.T) {
	rb := NewRingBuffer(5)
	got := rb.String()
	assertEqual(t, "", got)
	assertEqual(t, 0, rb.Len())
}

func TestBuffer_RingBuffer_String_Ugly(t *testing.T) {
	rb := NewRingBuffer(5)
	_, err := rb.Write([]byte("hello!"))
	requireNoError(t, err)
	assertEqual(t, "ello!", rb.String())
}

func TestBuffer_RingBuffer_Bytes_Good(t *testing.T) {
	rb := NewRingBuffer(5)
	_, err := rb.Write([]byte("hello"))
	requireNoError(t, err)
	assertEqual(t, []byte("hello"), rb.Bytes())
}

func TestBuffer_RingBuffer_Bytes_Bad(t *testing.T) {
	rb := NewRingBuffer(5)
	got := rb.Bytes()
	assertNil(t, got)
	assertEqual(t, 0, rb.Len())
}

func TestBuffer_RingBuffer_Bytes_Ugly(t *testing.T) {
	rb := NewRingBuffer(5)
	_, err := rb.Write([]byte("hello!"))
	requireNoError(t, err)
	assertEqual(t, []byte("ello!"), rb.Bytes())
}

func TestBuffer_RingBuffer_Len_Good(t *testing.T) {
	rb := NewRingBuffer(5)
	_, err := rb.Write([]byte("abc"))
	requireNoError(t, err)
	assertEqual(t, 3, rb.Len())
}

func TestBuffer_RingBuffer_Len_Bad(t *testing.T) {
	rb := NewRingBuffer(5)
	got := rb.Len()
	assertEqual(t, 0, got)
	assertEqual(t, "", rb.String())
}

func TestBuffer_RingBuffer_Len_Ugly(t *testing.T) {
	rb := NewRingBuffer(2)
	_, err := rb.Write([]byte("abcd"))
	requireNoError(t, err)
	assertEqual(t, 2, rb.Len())
}

func TestBuffer_RingBuffer_Cap_Good(t *testing.T) {
	rb := NewRingBuffer(7)
	got := rb.Cap()
	assertEqual(t, 7, got)
	assertEqual(t, 0, rb.Len())
}

func TestBuffer_RingBuffer_Cap_Bad(t *testing.T) {
	rb := NewRingBuffer(-7)
	got := rb.Cap()
	assertEqual(t, 0, got)
	assertEqual(t, "", rb.String())
}

func TestBuffer_RingBuffer_Cap_Ugly(t *testing.T) {
	rb := NewRingBuffer(1)
	_, err := rb.Write([]byte("xy"))
	requireNoError(t, err)
	assertEqual(t, 1, rb.Cap())
}

func TestBuffer_RingBuffer_Reset_Good(t *testing.T) {
	rb := NewRingBuffer(5)
	_, err := rb.Write([]byte("abc"))
	requireNoError(t, err)
	rb.Reset()
	assertEqual(t, "", rb.String())
}

func TestBuffer_RingBuffer_Reset_Bad(t *testing.T) {
	rb := NewRingBuffer(5)
	rb.Reset()
	assertEqual(t, 0, rb.Len())
	assertNil(t, rb.Bytes())
}

func TestBuffer_RingBuffer_Reset_Ugly(t *testing.T) {
	rb := NewRingBuffer(0)
	rb.Reset()
	n, err := rb.Write([]byte("abc"))
	requireNoError(t, err)
	assertEqual(t, 3, n)
}
