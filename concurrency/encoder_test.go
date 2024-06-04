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

func TestSimpleEncode(t *testing.T) {
	input := testStruct{Name: "foo", Value: 42}

	bufEnc, bufDec := "", ""
	ec := NewEncoderChain(JSONEncoder).PostFn(func(rw *ReadWriter) error {
		bufEnc = "executed encode post-function"
		var res testStruct
		dc := NewDecoderChain(rw, JSONDecoder).PostFn(func(rw *ReadWriter) error {
			bufDec = "executed decode post-function"
			return nil
		}).Build()
		require.Nil(t, dc.DecodeAndClose(&res))
		require.EqualValues(t, bufDec, "executed decode post-function")
		require.EqualValues(t, input, res)

		return nil
	}).Build()
	require.Nil(t, ec.EncodeAndClose(input))
	require.EqualValues(t, bufEnc, "executed encode post-function")

}

func TestEncoderChain(t *testing.T) {
	input := testStruct{Name: "foo", Value: 42}

	ec := NewEncoderChain(JSONEncoder).AddWriter(GZIPWriter).PostFn(func(rw *ReadWriter) error {
		var res testStruct
		dc := NewDecoderChain(rw, JSONDecoder).AddReader(GZIPReader).Build()
		require.Nil(t, dc.DecodeAndClose(&res))

		require.EqualValues(t, input, res)
		return nil
	}).Build()
	require.Nil(t, ec.EncodeAndClose(input))

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
			ec := NewEncoderChain(JSONEncoder).AddWriter(GZIPWriter).Build()

			_ = ec.Encode(input)
			_ = ec.Close()
		}
	})

}
