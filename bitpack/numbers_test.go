package bitpack

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	maxUint32 = 1<<32 - 1 // 4294967295
	maxUint64 = 1<<64 - 1 // 18446744073709551615
)

func TestEncodeDecodeUint64(t *testing.T) {
	for _, val := range []uint64{
		0,
		1,
		100,
		1000,
		10000,
		maxUint32,
		maxUint64,
	} {

		// Encode the number into a shortened string
		enc := EncodeUint64ToString(val)
		require.LessOrEqual(t, len(enc), len(fmt.Sprintf("%d", val)))

		// Encode the number into a byte buffer and verify consistency
		encBytes := make([]byte, stringEncUint64MaxBytes)
		n := EncodeUint64ToByteBuf(val, encBytes)
		require.EqualValues(t, enc, string(encBytes[0:n]))

		// Decode the key and verify consistency with input
		dec := DecodeUint64FromString(enc)
		require.Equal(t, val, dec)
	}
}

func TestInvalidDecodeUint64(t *testing.T) {
	for _, val := range []string{
		"",
		"Z",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	} {

		// Ensure that decoding doesn't panic
		require.NotPanics(t, func() {
			_ = DecodeUint64FromString(val)
		})
	}
}

// Test package level variables to avoid compiler optimizations in benchmarks
var (
	benchNum uint64
	benchStr string
)

func BenchmarkEncodeDecodeUint64(b *testing.B) {
	b.Run("encode", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchStr = EncodeUint64ToString(maxUint64)
		}
	})

	b.Run("encode_buffer", func(b *testing.B) {
		buf := make([]byte, 16)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			benchStr = EncodeUint64ToStringBuf(maxUint64, buf)
		}
	})

	b.Run("decode", func(b *testing.B) {
		benchStr = EncodeUint64ToString(maxUint64)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			benchNum = DecodeUint64FromString(benchStr)
		}
	})
}
