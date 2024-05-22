package concurrency

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	nTestLockCycles = 250

	lockFastDelay = time.Microsecond
	lockSlowDelay = time.Millisecond
)

func TestSimpleLock(t *testing.T) {

	t.Run("single", func(t *testing.T) {
		tpl := NewThreePointLock()

		ctx, cancel := context.WithCancel(context.Background())
		go loop(ctx, tpl)

		for i := 0; i < nTestLockCycles; i++ {
			err := tpl.Lock()
			require.Nil(t, err)

			require.Nil(t, tpl.Unlock())
		}
		cancel()
	})

	for _, nConc := range []int{2, 3, 5, 10, 100} {
		t.Run(fmt.Sprintf("multiple_%d", nConc), func(t *testing.T) {
			tpl := NewThreePointLock(WithMemPool(NewMemPoolLimitUnique(nConc, 1)))

			ctx, cancel := context.WithCancel(context.Background())

			var wg sync.WaitGroup
			for i := 0; i < nConc; i++ {
				go loop(ctx, tpl)

				wg.Add(1)
				go func() {
					for i := 0; i < nTestLockCycles; i++ {
						err := tpl.Lock()
						require.Nil(t, err)

						require.Nil(t, tpl.Unlock())
					}
					wg.Done()
				}()
			}

			wg.Wait()
			cancel()
		})
	}
}

func loop(ctx context.Context, tpl *ThreePointLock) {
	for {
		if tpl.HasLockRequest() {
			sem := tpl.ConsumeLockRequest()
			tpl.ConfirmLockRequest()

			for {
				if tpl.HasUnlockRequest() {
					break
				}
				time.Sleep(lockSlowDelay)
			}

			tpl.ConsumeUnlockRequest()
			tpl.Release(sem)

			tpl.Release(sem) // Duplicate release, should always be safe to do
		}

		time.Sleep(lockFastDelay)

		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}
