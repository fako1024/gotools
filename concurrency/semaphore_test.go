package concurrency

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSemaphore(t *testing.T) {

	sem := New(2)
	require.Zero(t, len(sem))
	require.Equal(t, 2, cap(sem))

	get1, err := sem.TryAdd()
	require.Nil(t, err)
	require.NotNil(t, get1)
	get2, err := sem.TryAdd()
	require.Nil(t, err)
	require.NotNil(t, get2)
	get3, err := sem.TryAdd()
	require.ErrorIs(t, err, ErrNoSlotAvailable)
	require.Nil(t, get3)

	start := time.Now()
	getTimeout, err := sem.TryAddFor(100 * time.Millisecond)
	require.ErrorIs(t, err, ErrNoSlotAvailable)
	require.Nil(t, getTimeout)
	require.GreaterOrEqual(t, time.Since(start), 100*time.Millisecond)

	get2()
	get2, err = sem.TryAdd()
	require.Nil(t, err)
	require.NotNil(t, get2)

	get2()
	getNoTimeout, err := sem.TryAddFor(100 * time.Millisecond)
	require.Nil(t, err)
	require.NotNil(t, getNoTimeout)

	getNoTimeout()
	get1()

	require.Zero(t, len(sem))
}
