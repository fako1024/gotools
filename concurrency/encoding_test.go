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

	wc := NewWriterChain().PostFn(func(rw *ReadWriter) error {
		var res testStruct
		rc := NewReaderChain(rw).PostFn(func(rw *ReadWriter) error {
			return nil
		}).Build()
		require.Nil(t, rc.DecodeAndClose(JSONDecoder, &res))
		require.EqualValues(t, input, res)

		return nil
	}).Build()
	require.Nil(t, wc.EncodeAndClose(JSONEncoder, input))
}

func TestEncoderChain(t *testing.T) {
	input := testStruct{Name: "foo", Value: 42}
	ref, err := encodeManual(input)
	require.Nil(t, err)

	// Repeat test a couple of times to trigger pool re-use scenario
	for i := 0; i < 100; i++ {
		wc := NewWriterChain().AddWriter(NewGZIPWriter()).PostFn(func(rw *ReadWriter) error {
			var res testStruct
			require.Equal(t, ref, rw.BytesCopy())

			dc := NewReaderChain(rw).AddReader(NewGZIPReader()).Build()
			require.Nil(t, dc.DecodeAndClose(JSONDecoder, &res))

			require.EqualValues(t, input, res)
			return nil
		}).Build()
		require.Nil(t, wc.EncodeAndClose(JSONEncoder, input))
	}
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
			wc := NewWriterChain().AddWriter(NewGZIPWriter()).Build()
			_ = wc.EncodeAndClose(JSONEncoder, input)
		}
	})

}

func encodeManual(input any) ([]byte, error) {
	enc, err := jsoniter.Marshal(input)
	if err != nil {
		return nil, err
	}

	// Append newline to mimic behavior of Encoder
	// https://github.com/golang/go/issues/37083
	enc = append(enc, []byte("\n")...)

	buf := bytes.NewBuffer(nil)
	gzw := gzip.NewWriter(buf)
	if _, err = gzw.Write(enc); err != nil {
		return nil, err
	}
	if err := gzw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
