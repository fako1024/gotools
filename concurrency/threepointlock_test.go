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
		var wgLoop = &sync.WaitGroup{}
		wgLoop.Add(1)
		go loop(ctx, tpl, wgLoop)

		for i := 0; i < nTestLockCycles; i++ {
			err := tpl.Lock()
			require.Nil(t, err)
			require.Nil(t, tpl.Unlock())
		}
		cancel()
		wgLoop.Wait()
	})

	for _, nConc := range []int{2, 3, 5, 10, 100} {
		t.Run(fmt.Sprintf("multiple_%d", nConc), func(t *testing.T) {
			memPool := NewMemPoolLimitUnique(nConc, 1)
			ctx, cancel := context.WithCancel(context.Background())

			var wg, wgLoop = &sync.WaitGroup{}, &sync.WaitGroup{}
			wg.Add(nConc)
			wgLoop.Add(nConc)
			for i := 0; i < nConc; i++ {
				tpl := NewThreePointLock(WithMemPool(memPool))

				go loop(ctx, tpl, wgLoop)
				go func(tpl *ThreePointLock) {
					for i := 0; i < nTestLockCycles; i++ {
						tpl.MustLock()
						tpl.MustUnlock()
					}
					wg.Done()
				}(tpl)
			}

			wg.Wait()
			cancel()
			wgLoop.Wait()
			memPool.Clear()
		})
	}
}

func loop(ctx context.Context, tpl *ThreePointLock, wg *sync.WaitGroup) {
	defer func() {
		wg.Done()
		tpl.Close()
	}()
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
