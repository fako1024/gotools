package concurrency

import (
	"io"
	"io/fs"
	"sync"
	"unsafe"
)

var defaultMemPool = NewMemPoolNoLimit()

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

	// Slice element Get / Put operations
	Get(size int) (elem []byte)
	Put(elem []byte)

	// io.ReadWriter Get / Put operations
	GetReadWriter(size int) *ReadWriter
	PutReadWriter(elem *ReadWriter)
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

// GetReadWriter return a wrapped element providing an io.ReadWriter
func (p *MemPoolLimit) GetReadWriter(size int) *ReadWriter {
	return &ReadWriter{
		data: p.Get(size),
	}
}

// PutReadWriter returns a wrapped element providing an io.ReadWriter to the pool
func (p *MemPoolLimit) PutReadWriter(elem *ReadWriter) {
	p.Put(elem.data)
}

// Clear releases all pool resources and makes them available for garbage collection
func (p *MemPoolLimit) Clear() {
	p.elements = nil
}

// MemPoolLimitUnique provides a channel-based memory buffer pool (limiting the number
// of resources, enforcing their uniqueness and allowing for cleanup)
type MemPoolLimitUnique struct {
	elements           chan []byte
	tracker            map[uintptr]bool
	initialElementSize int

	sync.Mutex
}

// NewMemPoolLimitUnique instantiates a new memory pool that manages bytes slices
func NewMemPoolLimitUnique(n int, initialElementSize int) *MemPoolLimitUnique {
	obj := MemPoolLimitUnique{
		elements:           make(chan []byte, n),
		tracker:            make(map[uintptr]bool),
		initialElementSize: initialElementSize,
	}
	for i := 0; i < n; i++ {
		elem := make([]byte, initialElementSize)

		obj.elements <- elem
		obj.tracker[slicePtr(elem)] = false // track as non-taken
	}

	return &obj
}

// Get retrieves a memory element (already performing the type assertion)
func (p *MemPoolLimitUnique) Get(size int) (elem []byte) {

	elem = <-p.elements

	p.Lock()
	if cap(elem) < size {
		delete(p.tracker, slicePtr(elem))
		elem = make([]byte, size*2)
		p.tracker[slicePtr(elem)] = false
	}
	p.tracker[slicePtr(elem)] = true // track as taken
	p.Unlock()

	elem = elem[:size]

	return
}

// Put returns a memory element to the pool, resetting its size to capacity
// in the process
func (p *MemPoolLimitUnique) Put(elem []byte) {

	elem = elem[:cap(elem)]

	p.Lock()
	taken, exists := p.tracker[slicePtr(elem)]
	if !exists {
		panic("cannot return untracked memory element to pool")
	}

	p.tracker[slicePtr(elem)] = false // track as non-taken
	p.Unlock()

	// If the tracked element isn't taken this is probably a duplicate Put()
	// operation and we ignore it to avoid potential deadlocks on the memory
	// pool channel
	if !taken {
		return
	}

	p.elements <- elem
}

// Resize resizes an element of the pool, updating its tracking information
// in the process
func (p *MemPoolLimitUnique) Resize(elem []byte, size int) []byte {

	p.Lock()
	ptr := slicePtr(elem)
	if _, exists := p.tracker[ptr]; !exists {
		panic("cannot resize untracked memory element")
	}

	if cap(elem) < size {
		newElem := make([]byte, size)
		copy(newElem, elem)

		delete(p.tracker, ptr)
		p.tracker[slicePtr(newElem)] = true
		p.Unlock()
		return newElem
	}

	elem = elem[:size]
	p.tracker[ptr] = true
	p.Unlock()

	return elem
}

// Clear releases all pool resources and makes them available for garbage collection
func (p *MemPoolLimitUnique) Clear() {
	p.elements = nil
	p.tracker = nil
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

// GetReadWriter returns a wrapped element providing an io.ReadWriter
func (p *MemPoolNoLimit) GetReadWriter(size int) *ReadWriter {
	return &ReadWriter{
		data: p.Get(size),
	}
}

// PutReadWriter returns a wrapped element providing an io.ReadWriter to the pool
func (p *MemPoolNoLimit) PutReadWriter(elem *ReadWriter) {
	p.Put(elem.data)
}

// Helper function to get the pointer to the first element in a slice, to be
// used as key for uniqueness tracking
func slicePtr(elem []byte) uintptr {
	return uintptr(unsafe.Pointer(&elem[0])) // #nosec G103
}
