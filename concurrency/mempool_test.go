package concurrency

import (
	"fmt"
	"testing"
)

func TestAllocateALot(t *testing.T) {

	bufs := make([][]byte, 0)

	pool := NewMemPoolNoLimit()
	for i := 0; i < 10; i++ {
		buf := pool.Get(10 * 1024 * 1024)
		fmt.Printf("%p\n", buf)
		bufs = append(bufs, buf)
	}

	_ = bufs
}
