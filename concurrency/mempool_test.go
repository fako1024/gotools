package concurrency

import (
	"compress/gzip"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAllocateALot(t *testing.T) {

	bufs := make([][]byte, 0)

	pool := NewMemPoolNoLimit()
	for i := 0; i < 10; i++ {
		buf := pool.Get(10 * 1024 * 1024)
		bufs = append(bufs, buf)
	}

	_ = bufs
}

func TestReaderWriter(t *testing.T) {

	for _, pool := range []MemPool{
		NewMemPoolNoLimit(),
		NewMemPool(2),
	} {

		maxTestInputLen := 0
		for _, testInput := range []string{
			"",
			"a",
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.",
		} {
			elem := pool.GetReadWriter(0)

			// Encoding
			zipper := gzip.NewWriter(elem)
			n, err := zipper.Write([]byte(testInput))
			if n > maxTestInputLen {
				maxTestInputLen = n
			}
			require.Nil(t, err)
			require.Equal(t, len(testInput), n)
			require.Nil(t, zipper.Close())

			// Decoding
			unzipper, err := gzip.NewReader(elem)
			require.Nil(t, err)
			output := pool.GetReadWriter(0)
			_, err = io.Copy(output, unzipper) // #nosec G110
			require.Nil(t, err)

			require.Equal(t, testInput, string(output.data))

			pool.PutReadWriter(elem)
			pool.PutReadWriter(output)
		}

		// There should be two elements in the pool, both of which should return length zero and a capacity
		// higher than the initial / minimal one (indicating that the have been used / grown before)
		elem1, elem2 := pool.GetReadWriter(0), pool.GetReadWriter(0)
		require.Zero(t, len(elem1.data))
		require.Zero(t, len(elem2.data))
	}
}
