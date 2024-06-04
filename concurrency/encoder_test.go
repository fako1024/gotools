package concurrency

import (
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/require"
)

type testStruct struct {
	Name  string
	Value int
}

// Prototype / use case: https://github.com/open-telemetry/opentelemetry-collector/blob/4bbb60402f262214aecacb24839e75159143a43f/receiver/otlpreceiver/otlphttp.go#L58

func TestSimpleEncode(t *testing.T) {
	input := testStruct{Name: "foo", Value: 42}
	output := bytes.NewBuffer([]byte{})

	buf := ""
	ec := NewEncoderChain(output, JSONEncoder).PostFn(func(w io.Writer) error {
		buf = "executed encode post-function"
		return nil
	}).Build()
	require.Nil(t, ec.EncodeAndClose(input))
	require.EqualValues(t, buf, "executed encode post-function")

	var res testStruct
	dc := NewDecoderChain(output, JSONDecoder).PostFn(func(r io.Reader) error {
		buf = "executed decode post-function"
		return nil
	}).Build()
	require.Nil(t, dc.DecodeAndClose(&res))
	require.EqualValues(t, buf, "executed decode post-function")

	require.EqualValues(t, input, res)
}

func TestEncoderChain(t *testing.T) {
	input := testStruct{Name: "foo", Value: 42}
	output := bytes.NewBuffer([]byte{})

	ec := NewEncoderChain(output, JSONEncoder).AddWriter(GZIPWriter).Build()
	require.Nil(t, ec.EncodeAndClose(input))

	var res testStruct
	dc := NewDecoderChain(output, JSONDecoder).AddReader(GZIPReader).Build()
	require.Nil(t, dc.DecodeAndClose(&res))

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
