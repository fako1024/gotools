package concurrency

import (
	"io"
	"io/fs"
	"sync"
)

// ReadWriteSeekCloser provides an interface to all the wrapped interfaces
// in one instance
type ReadWriteSeekCloser interface {
	Stat() (fs.FileInfo, error)

	io.Reader
	io.Writer
	io.Seeker
	io.Closer
}

// MemPool denotes a generic memory buffer pool
type MemPool interface {
	Get(size int) (elem []byte)
	Put(elem []byte)
}

// MemPoolGCable denotes a generic memory buffer pool that can be "cleaned", i.e.
// that allows making all resources available for garbage collection
type MemPoolGCable interface {
	Clear()

	MemPool
}

// MemPoolLimit provides a channel-based memory buffer pool (limiting the number
// of resources and allowing for cleanup)
type MemPoolLimit struct {
	elements chan []byte
}

// NewMemPool instantiates a new memory pool that manages bytes slices
func NewMemPool(n int) *MemPoolLimit {
	obj := MemPoolLimit{
		elements: make(chan []byte, n),
	}
	for i := 0; i < n; i++ {
		obj.elements <- make([]byte, 0)
	}
	return &obj
}

// Get retrieves a memory element (already performing the type assertion)
func (p *MemPoolLimit) Get(size int) (elem []byte) {
	elem = <-p.elements
	if cap(elem) < size {
		elem = make([]byte, size*2)
	}
	elem = elem[:size]
	return
}

// Put returns a memory element to the pool, resetting its size to capacity
// in the process
func (p *MemPoolLimit) Put(elem []byte) {
	elem = elem[:cap(elem)]
	p.elements <- elem
}

// Clear releases all pool resources and makes them available for garbage collection
func (p *MemPoolLimit) Clear() {
	p.elements = nil
}

// MemPoolNoLimit wraps a standard sync.Pool (no limit to resources)
type MemPoolNoLimit struct {
	sync.Pool
}

// NewMemPoolNoLimit instantiates a new memory pool that manages bytes slices
// of arbitrary capacity
func NewMemPoolNoLimit() *MemPoolNoLimit {
	return &MemPoolNoLimit{
		Pool: sync.Pool{
			New: func() any {
				return make([]byte, 0)
			},
		},
	}
}

// Get retrieves a memory element (already performing the type assertion)
func (p *MemPoolNoLimit) Get(size int) (elem []byte) {
	elem = p.Pool.Get().([]byte)
	if cap(elem) < size {
		elem = make([]byte, size*2)
	}
	elem = elem[:size]
	return
}

// Put returns a memory element to the pool, resetting its size to capacity
// in the process
func (p *MemPoolNoLimit) Put(elem []byte) {
	elem = elem[:cap(elem)]

	// nolint:staticcheck
	p.Pool.Put(elem)
}
