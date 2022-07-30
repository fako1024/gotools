package concurrency

import "sync"

const (
	NoLimit = 0 // NoLimit denotes no limit imposed on the concurrency
)

// Limit provides a generic concurrency / work limiter
type Limit struct {
	limiter chan struct{}
	mutex   sync.Mutex
}

// New instantiates a new limiter with the given maximum concurrency
func New(n int) (l *Limit) {
	l = &Limit{
		mutex: sync.Mutex{},
	}
	if n > 0 {
		l.limiter = make(chan struct{}, n)
	}
	return
}

// Add adds a new worker / task to be taken into account
func (l *Limit) Add() {
	if cap(l.limiter) > 0 {
		l.limiter <- struct{}{}
	}
}

// Done releases a worker / task back into the pool
func (l *Limit) Done() {
	if cap(l.limiter) > 0 {
		<-l.limiter
	}
}
