package concurrency

import (
	"errors"
	"fmt"
	"time"
)

var (

	// ErrLockConfirmTimeout signifies that the lock request has not been confirmed
	// by the main routine (in a timely manner)
	ErrLockConfirmTimeout = errors.New("timeout waiting for lock confirmation")
)

// Semaphore is simply the underlying byte slice (from a memory pool), serving
// as both shared buffer and Semaphore representing the held lock
type Semaphore = []byte

// ThreePointLock denotes a concurrency pattern that allows for rare locks on a high-
// throughput main loop function with minimal performance impact on said routine
type ThreePointLock struct {

	// Core channels used to facilitate the atomic three-point operation
	request chan []byte
	confirm chan struct{}
	done    chan struct{}

	// Optional functions to be executed as part of the locking process
	// to signal the main loop routine
	lockRequestFn   func() error
	unlockRequestFn func() error

	// Timeout for lock operation
	timeout time.Duration

	// Memory pool
	memPool        *MemPoolLimitUnique
	minElementSize int
}

// ThreePointLockOption denotes a functional option for the three-point lock type
type ThreePointLockOption func(*ThreePointLock)

// WithMemPool sets the memory pool for the ThreePointLock
func WithMemPool(memPool *MemPoolLimitUnique) ThreePointLockOption {
	return func(tpl *ThreePointLock) {
		tpl.memPool = memPool
	}
}

// WithLockRequestFn sets the lock request function for the ThreePointLock
func WithLockRequestFn(fn func() error) ThreePointLockOption {
	return func(tpl *ThreePointLock) {
		tpl.lockRequestFn = fn
	}
}

// WithUnlockRequestFn sets the unlock request function for the ThreePointLock
func WithUnlockRequestFn(fn func() error) ThreePointLockOption {
	return func(tpl *ThreePointLock) {
		tpl.unlockRequestFn = fn
	}
}

// WithTimeout sets the timeout for the lock operation in the ThreePointLock
func WithTimeout(timeout time.Duration) ThreePointLockOption {
	return func(tpl *ThreePointLock) {
		tpl.timeout = timeout
	}
}

// WithMinElementSize sets the minimum element size for the ThreePointLock
func WithMinElementSize(size int) ThreePointLockOption {
	return func(tpl *ThreePointLock) {
		tpl.minElementSize = size
	}
}

// NewThreePointLock creates a new instance of ThreePointLock with the given options
func NewThreePointLock(options ...ThreePointLockOption) *ThreePointLock {
	obj := &ThreePointLock{
		request:        make(chan []byte, 1),
		confirm:        make(chan struct{}),
		done:           make(chan struct{}, 1),
		minElementSize: 1, // Should be greater than zero, otherwise slice pointer access will fail
	}

	// Apply functional options (if present)
	for _, opt := range options {
		opt(obj)
	}

	// By default, initialize a memory pool that does not allow any
	// concurrent lock access (in case none has been provided via option)
	if obj.memPool == nil {
		obj.memPool = NewMemPoolLimitUnique(1, obj.minElementSize)
	}

	return obj
}

// Lock acquires the lock and returns the semaphore
// If a timeout is specified, the method waits until the timeout expires
func (tpl *ThreePointLock) Lock() (err error) {

	// Fetch data from the pool to establish a claim (will wait until it is actually
	// available)
	sem := tpl.memPool.Get(tpl.memPool.initialElementSize)

	// Notify the main routine that a locked interaction is about to begin
	tpl.request <- sem

	// Execute optional pre-lock function (e.g. an unblock command or similar)
	if tpl.lockRequestFn != nil {
		if err = tpl.lockRequestFn(); err != nil {
			tpl.memPool.Put(sem) // Return semaphore on failure
			return
		}
	}

	// Wait for confirmation of reception from the processing routine...

	// If no timeout has been specified, wait forever
	if tpl.timeout == 0 {
		<-tpl.confirm
		return
	}

	// If a timeout has been specified, wait until it expires
	select {
	case <-tpl.confirm:
		return
	case <-time.After(tpl.timeout):
		err = ErrLockConfirmTimeout
		tpl.memPool.Put(sem) // Return semaphore on failure
		return
	}
}

// MustLock acquires the lock and returns the semaphore (panics on failure)
func (tpl *ThreePointLock) MustLock() {
	if err := tpl.Lock(); err != nil {
		panic(fmt.Sprintf("failed to establish three-point lock: %s", err))
	}
}

// Unlock releases the lock
func (tpl *ThreePointLock) Unlock() (err error) {

	// Signal that the lock is complete / done, releasing the main routine
	tpl.done <- struct{}{}

	// Execute optional post-lock function (e.g. an unblock command or similar)
	if tpl.unlockRequestFn != nil {
		if err = tpl.unlockRequestFn(); err != nil {
			return
		}
	}

	return
}

// MustUnlock releases the lock (panics on failure)
func (tpl *ThreePointLock) MustUnlock() {
	if err := tpl.Unlock(); err != nil {
		panic(fmt.Sprintf("failed to release three-point lock: %s", err))
	}
}

// HasLockRequest checks if there is a lock request
func (tpl *ThreePointLock) HasLockRequest() bool {
	return len(tpl.request) > 0
}

// ConsumeLockRequest consumes the lock request
func (tpl *ThreePointLock) ConsumeLockRequest() Semaphore {
	return <-tpl.request
}

// HasUnlockRequest checks if there is an unlock request
func (tpl *ThreePointLock) HasUnlockRequest() bool {
	return len(tpl.done) > 0
}

// ConsumeUnlockRequest consumes the unlock request
func (tpl *ThreePointLock) ConsumeUnlockRequest() {
	<-tpl.done
}

// ConfirmLockRequest confirms that the main loop is not processing
func (tpl *ThreePointLock) ConfirmLockRequest() {
	tpl.confirm <- struct{}{}
}

// Release releases the semaphore back to the memory pool
func (tpl *ThreePointLock) Release(sem Semaphore) {
	tpl.memPool.Put(sem)
}

// Closes ensures that all channels are closed, releasing any potentially waiting goroutines
func (tpl *ThreePointLock) Close() {
	close(tpl.request)
	close(tpl.confirm)
	close(tpl.done)
}
