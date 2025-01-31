package concurrency

import (
	"context"
	"errors"
	"time"
)

const (
	NoLimit = 0 // NoLimit denotes no limit imposed on the concurrency
)

var (
	//ErrNoSlotAvailable denotes that there is no slot available at present
	ErrNoSlotAvailable = errors.New("no semaphore slot available")
)

// Semaphore provides a generic concurrency / work semaphore
type Semaphore chan struct{}

// Limt provides backward compatibility and an alternative naming scheme for
// the Semaphore type
type Limit = Semaphore

// New instantiates a new semaphore with the given maximum concurrency
func New(n int) (l Semaphore) {
	if n > 0 {
		l = make(chan struct{}, n)
	}
	return
}

// Add adds a new worker / task to be taken into account
func (l Semaphore) Add() {
	if cap(l) > 0 {
		l <- struct{}{}
	}
}

// Done releases a worker / task back into the pool
func (l Semaphore) Done() {
	if cap(l) > 0 {
		<-l
	}
}

// TryAdd attempts to add a new worker / task to be taken into account and aborts
// with an error if not possible
func (l Semaphore) TryAdd() (func(), error) {
	// Try to acquire a slot
	select {
	// If semaphore is available, return done function
	case l <- struct{}{}:
		return func() { <-l }, nil
	// If none is available, return nothing and a sentinel error
	default:
		return nil, ErrNoSlotAvailable
	}
}

// TryAddFor attempts to add a new worker / task to be taken into account for
// a certain period of time, otherwise aborts with an error
func (l Semaphore) TryAddFor(timeout time.Duration) (func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Try to acquire a slot
	select {
	// If semaphore becomes available within timeout, return done function
	case l <- struct{}{}:
		return func() { <-l }, nil
	// If timeout ensues, return nothing and a sentinel error
	case <-ctx.Done():
		return nil, ErrNoSlotAvailable
	}
}
