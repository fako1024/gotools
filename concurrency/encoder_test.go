package concurrency

import (
	"bytes"
	"compress/gzip"
	"testing"

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/require"
)

type testStruct struct {
	Name  string
	Value int
}

// Prototype / use case: https://github.com/open-telemetry/opentelemetry-collector/blob/4bbb60402f262214aecacb24839e75159143a43f/receiver/otlpreceiver/otlphttp.go#L58

func TestEncoderChain(t *testing.T) {

	input := testStruct{Name: "foo", Value: 42}
	output := bytes.NewBuffer(nil)

	ec := NewEncoderChain(output, JSONEncoder).AddWriter(GZIPWriter).Build()
	require.Nil(t, ec.Encode(input))
	require.Nil(t, ec.Close())

	var res testStruct
	dc := NewDecoderChain(output, JSONDecoder).AddReader(GZIPReader).Build()
	require.Nil(t, dc.Decode(&res))
	require.Nil(t, dc.Close())

	require.EqualValues(t, input, res)
}

func BenchmarkEncoderChain(b *testing.B) {
	input := testStruct{Name: "foo", Value: 42}

	b.Run("classic", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			output := bytes.NewBuffer(nil)

			enc, _ := jsoniter.Marshal(input)
			gz := gzip.NewWriter(output)
			_, _ = gz.Write(enc)
			gz.Close()

		}
	})

	b.Run("chain", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			output := bytes.NewBuffer(nil)
			ec := NewEncoderChain(output, JSONEncoder).AddWriter(GZIPWriter).Build()

			_ = ec.Encode(input)
			_ = ec.Close()
		}
	})

}
