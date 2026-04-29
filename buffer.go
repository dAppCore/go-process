package process

import (
	// Note: AX-6 — internal concurrency primitive; structural per RFC §2
	"sync"

	core "dappco.re/go"
)

// RingBuffer is a fixed-size circular buffer that overwrites old data.
// Thread-safe for concurrent reads and writes.
type RingBuffer struct {
	data  []byte
	size  int
	start int
	end   int
	full  bool
	mu    sync.RWMutex
}

// NewRingBuffer creates a ring buffer with the given capacity.
func NewRingBuffer(size int) *RingBuffer {
	if size < 0 {
		size = 0
	}
	return &RingBuffer{
		data: make([]byte, size),
		size: size,
	}
}

// Write appends data to the buffer, overwriting oldest data if full.
func (rb *RingBuffer) Write(p []byte) core.Result {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.size == 0 {
		return core.Ok(len(p))
	}

	for _, b := range p {
		rb.data[rb.end] = b
		rb.end = (rb.end + 1) % rb.size
		if rb.full {
			rb.start = (rb.start + 1) % rb.size
		}
		if rb.end == rb.start {
			rb.full = true
		}
	}
	return core.Ok(len(p))
}

// String returns the buffer contents as a string.
func (rb *RingBuffer) String() string {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if !rb.full && rb.start == rb.end {
		return ""
	}

	if rb.full {
		result := make([]byte, rb.size)
		copy(result, rb.data[rb.start:])
		copy(result[rb.size-rb.start:], rb.data[:rb.end])
		return string(result)
	}

	return string(rb.data[rb.start:rb.end])
}

// Bytes returns a copy of the buffer contents.
func (rb *RingBuffer) Bytes() []byte {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if !rb.full && rb.start == rb.end {
		return nil
	}

	if rb.full {
		result := make([]byte, rb.size)
		copy(result, rb.data[rb.start:])
		copy(result[rb.size-rb.start:], rb.data[:rb.end])
		return result
	}

	result := make([]byte, rb.end-rb.start)
	copy(result, rb.data[rb.start:rb.end])
	return result
}

// Len returns the current length of data in the buffer.
func (rb *RingBuffer) Len() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.full {
		return rb.size
	}
	if rb.end >= rb.start {
		return rb.end - rb.start
	}
	return rb.size - rb.start + rb.end
}

// Cap returns the buffer capacity.
func (rb *RingBuffer) Cap() int {
	return rb.size
}

// Reset clears the buffer.
func (rb *RingBuffer) Reset() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.start = 0
	rb.end = 0
	rb.full = false
}
