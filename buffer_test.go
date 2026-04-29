package process

import (
	"testing"
)

func TestRingBufferBasics(t *testing.T) {
	t.Run("write and read", func(t *testing.T) {
		rb := NewRingBuffer(10)

		result := rb.Write([]byte("hello"))
		assertNoError(t, result)
		assertEqual(t, 5, result.Value.(int))
		assertEqual(t, "hello", rb.String())
		assertEqual(t, 5, rb.Len())
	})

	t.Run("overflow wraps around", func(t *testing.T) {
		rb := NewRingBuffer(5)

		assertNoError(t, rb.Write([]byte("hello")))
		assertEqual(t, "hello", rb.String())

		assertNoError(t, rb.Write([]byte("world")))
		// Should contain "world" (overwrote "hello")
		assertEqual(t, 5, rb.Len())
		assertEqual(t, "world", rb.String())
	})

	t.Run("partial overflow", func(t *testing.T) {
		rb := NewRingBuffer(10)

		assertNoError(t, rb.Write([]byte("hello")))
		assertNoError(t, rb.Write([]byte("worldx")))
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
		assertNoError(t, rb.Write([]byte("hello")))
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
		assertNoError(t, rb.Write([]byte("hello")))

		bytes := rb.Bytes()
		assertEqual(t, []byte("hello"), bytes)

		// Modifying returned bytes shouldn't affect buffer
		bytes[0] = 'x'
		assertEqual(t, "hello", rb.String())
	})

	t.Run("zero or negative capacity is a no-op", func(t *testing.T) {
		for _, size := range []int{0, -1} {
			rb := NewRingBuffer(size)

			result := rb.Write([]byte("discarded"))
			assertNoError(t, result)
			assertEqual(t, len("discarded"), result.Value.(int))
			assertEqual(t, 0, rb.Cap())
			assertEqual(t, 0, rb.Len())
			assertEqual(t, "", rb.String())
			assertNil(t, rb.Bytes())
		}
	})
}

func TestBuffer_NewRingBuffer_Good(t *testing.T) {
	rb := NewRingBuffer(4)
	result := rb.Write([]byte("go"))
	requireNoError(t, result)
	assertEqual(t, 2, result.Value.(int))
	assertEqual(t, 4, rb.Cap())
}

func TestBuffer_NewRingBuffer_Bad(t *testing.T) {
	rb := NewRingBuffer(-1)
	result := rb.Write([]byte("drop"))
	requireNoError(t, result)
	assertEqual(t, 0, rb.Cap())
	assertEqual(t, 0, rb.Len())
	assertEqual(t, len("drop"), result.Value.(int))
}

func TestBuffer_NewRingBuffer_Ugly(t *testing.T) {
	rb := NewRingBuffer(0)
	result := rb.Write(nil)
	requireNoError(t, result)
	assertEqual(t, 0, result.Value.(int))
	assertEqual(t, "", rb.String())
}

func TestBuffer_RingBuffer_Write_Good(t *testing.T) {
	rb := NewRingBuffer(8)
	result := rb.Write([]byte("abc"))
	requireNoError(t, result)
	assertEqual(t, 3, result.Value.(int))
	assertEqual(t, "abc", rb.String())
}

func TestBuffer_RingBuffer_Write_Bad(t *testing.T) {
	rb := NewRingBuffer(0)
	result := rb.Write([]byte("abc"))
	requireNoError(t, result)
	assertEqual(t, len("abc"), result.Value.(int))
	assertEqual(t, "", rb.String())
}

func TestBuffer_RingBuffer_Write_Ugly(t *testing.T) {
	rb := NewRingBuffer(3)
	result := rb.Write([]byte("abcdef"))
	requireNoError(t, result)
	assertEqual(t, 6, result.Value.(int))
	assertEqual(t, "def", rb.String())
}

func TestBuffer_RingBuffer_String_Good(t *testing.T) {
	rb := NewRingBuffer(5)
	requireNoError(t, rb.Write([]byte("hello")))
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
	requireNoError(t, rb.Write([]byte("hello!")))
	assertEqual(t, "ello!", rb.String())
}

func TestBuffer_RingBuffer_Bytes_Good(t *testing.T) {
	rb := NewRingBuffer(5)
	requireNoError(t, rb.Write([]byte("hello")))
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
	requireNoError(t, rb.Write([]byte("hello!")))
	assertEqual(t, []byte("ello!"), rb.Bytes())
}

func TestBuffer_RingBuffer_Len_Good(t *testing.T) {
	rb := NewRingBuffer(5)
	requireNoError(t, rb.Write([]byte("abc")))
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
	requireNoError(t, rb.Write([]byte("abcd")))
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
	requireNoError(t, rb.Write([]byte("xy")))
	assertEqual(t, 1, rb.Cap())
}

func TestBuffer_RingBuffer_Reset_Good(t *testing.T) {
	rb := NewRingBuffer(5)
	requireNoError(t, rb.Write([]byte("abc")))
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
	result := rb.Write([]byte("abc"))
	requireNoError(t, result)
	assertEqual(t, 3, result.Value.(int))
}
