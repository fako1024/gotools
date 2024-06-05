package concurrency

import (
	"io"
)

// minBufferSize is an initial allocation minimal capacity.
const minBufferSize = 64

// ReadWriter denotes a wrapper around a data slice from a memory pool that fulfils the
// io.Reader and io.Writer interfaces (similar to a bytes.Buffer, on which parts of the
// implementation are based on)
type ReadWriter struct {
	data   []byte
	offset int
}

// Read reads the next len(p) bytes from the buffer or until the buffer
// is drained. The return value n is the number of bytes read. If the
// buffer has no data to return, err is io.EOF (unless len(p) is zero);
// otherwise it is nil
func (rw *ReadWriter) Read(p []byte) (n int, err error) {
	if rw.empty() {
		if len(p) == 0 {
			return 0, nil
		}
		return 0, io.EOF
	}
	n = copy(p, rw.data[rw.offset:])
	rw.offset += n

	return n, nil
}

// Write appends the contents of p to the buffer, growing the buffer as
// needed. The return value n is the length of p; err is always nil
func (rw *ReadWriter) Write(p []byte) (int, error) {
	m := rw.grow(len(p))
	return copy(rw.data[m:], p), nil
}

// Bytes returns a slice holding the unread portion of the ReadWriter, valid for use only
// until the next buffer modification (that is, only until the next call to a method like
// Read(), Write() or Reset()
// The slice aliases the buffer content at least until the next buffer modification,
// so immediate changes to the slice will affect the result of future reads
func (rw *ReadWriter) Bytes() []byte { return rw.data[rw.offset:] }

// BytesCopy returns a slice holding a copy of the unread portion of the ReadWriter
func (rw *ReadWriter) BytesCopy() []byte { 
	buf := rw.Bytes()
	res := make([]byte, len(buf))
	copy(res, buf)
	return res
}

// Reset resets the buffer to be empty,
// but it retains the underlying storage for use by future writes
func (rw *ReadWriter) Reset() {
	rw.data = rw.data[:0]
	rw.offset = 0
}

// empty reports whether the unread portion of the buffer is empty.
func (rw *ReadWriter) empty() bool { return len(rw.data) <= rw.offset }

// Len returns the number of bytes of the unread portion of the buffer;
// b.Len() == len(b.Bytes()).
func (rw *ReadWriter) len() int { return len(rw.data) - rw.offset }

// grow grows the buffer to guarantee space for n more bytes.
// It returns the index where bytes should be written.
// If the buffer can't grow it will panic with ErrTooLarge.
func (rw *ReadWriter) grow(n int) int {
	m := rw.len()
	if m == 0 && rw.offset != 0 {
		rw.Reset()
	}

	if rw.data == nil && n <= minBufferSize {
		rw.data = make([]byte, n, minBufferSize)
		return 0
	}

	c := cap(rw.data)
	if n <= c/2-m {
		copy(rw.data, rw.data[rw.offset:])
	} else {
		rw.data = growSlice(rw.data[rw.offset:], rw.offset+n)
	}

	rw.offset = 0
	rw.data = rw.data[:m+n]
	return m
}

// growSlice grows b by n, preserving the original content of b.
// If the allocation fails, it panics with ErrTooLarge.
func growSlice(b []byte, n int) []byte {
	b2 := append([]byte(nil), make([]byte, len(b)+n)...)
	copy(b2, b)
	return b2[:len(b)]
}
