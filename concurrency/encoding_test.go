package concurrency

import (
	"bytes"
	"compress/gzip"
	"testing"

	jsoniter "github.com/json-iterator/go"
	yaml "gopkg.in/yaml.v3"

	"github.com/stretchr/testify/require"
)

type testStruct struct {
	Name  string
	Value int
}

type testCase struct {
	name       string
	encoder    EncoderFn
	decoder    DecoderFn
	refEncoder func(any) ([]byte, error)
}

func TestSimpleEncode(t *testing.T) {
	input := testStruct{Name: "foo", Value: 42}

	for _, cs := range []testCase{
		{
			name:    "JSON",
			encoder: JSONEncoder,
			decoder: JSONDecoder,
		},
		{
			name:    "YAML",
			encoder: YAMLEncoder,
			decoder: YAMLDecoder,
		},
	} {
		t.Run(cs.name, func(t *testing.T) {
			wc := NewWriterChain().PostFn(func(rw *ReadWriter) error {
				var res testStruct
				rc := NewReaderChain(rw).PostFn(func(rw *ReadWriter) error {
					return nil
				}).Build()
				require.Nil(t, rc.DecodeAndClose(cs.decoder, &res))
				require.EqualValues(t, input, res)

				return nil
			}).Build()
			require.Nil(t, wc.EncodeAndClose(cs.encoder, input))
		})
	}
}

func TestSimpleByteEncode(t *testing.T) {
	input := []byte("This is a test")

	wc := NewWriterChain().PostFn(func(rw *ReadWriter) error {
		var res []byte
		rc := NewReaderChain(rw).PostFn(func(rw *ReadWriter) error {
			return nil
		}).Build()
		require.Nil(t, rc.DecodeAndClose(BytesDecoder, &res))
		require.EqualValues(t, input, res)

		return nil
	}).Build()
	require.Nil(t, wc.EncodeAndClose(BytesEncoder, input))
}

func TestEncoderChain(t *testing.T) {
	input := testStruct{Name: "foo", Value: 42}

	for _, cs := range []testCase{
		{
			name:       "JSON",
			encoder:    JSONEncoder,
			decoder:    JSONDecoder,
			refEncoder: encodeManualJSON,
		},
		{
			name:       "YAML",
			encoder:    YAMLEncoder,
			decoder:    YAMLDecoder,
			refEncoder: encodeManualYAML,
		},
	} {
		t.Run(cs.name, func(t *testing.T) {
			ref, err := cs.refEncoder(input)
			require.Nil(t, err)

			// Repeat test a couple of times to trigger pool re-use scenario
			for i := 0; i < 100; i++ {
				wc := NewWriterChain().AddWriter(NewGZIPWriter()).PostFn(func(rw *ReadWriter) error {
					var res testStruct
					require.Equal(t, ref, rw.BytesCopy())

					dc := NewReaderChain(rw).AddReader(NewGZIPReader()).Build()
					require.Nil(t, dc.DecodeAndClose(cs.decoder, &res))

					require.EqualValues(t, input, res)
					return nil
				}).Build()
				require.Nil(t, wc.EncodeAndClose(cs.encoder, input))
			}
		})
	}
}

func TestEncoderChainBytes(t *testing.T) {
	input := []byte("This is a test")

	ref, err := gzipManual(input)
	require.Nil(t, err)

	// Repeat test a couple of times to trigger pool re-use scenario
	for i := 0; i < 100; i++ {
		wc := NewWriterChain().AddWriter(NewGZIPWriter()).PostFn(func(rw *ReadWriter) error {
			var res []byte
			require.Equal(t, ref, rw.BytesCopy())

			dc := NewReaderChain(rw).AddReader(NewGZIPReader()).Build()
			require.Nil(t, dc.DecodeAndClose(BytesDecoder, &res))

			require.EqualValues(t, input, res)
			return nil
		}).Build()
		require.Nil(t, wc.EncodeAndClose(BytesEncoder, input))
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
			_ = gz.Close()
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

func encodeManualJSON(input any) ([]byte, error) {
	enc, err := jsoniter.Marshal(input)
	if err != nil {
		return nil, err
	}

	// Append newline to mimic behavior of Encoder
	// https://github.com/golang/go/issues/37083
	enc = append(enc, []byte("\n")...)

	return gzipManual(enc)
}

func encodeManualYAML(input any) ([]byte, error) {
	enc, err := yaml.Marshal(input)
	if err != nil {
		return nil, err
	}

	return gzipManual(enc)
}

func gzipManual(input []byte) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	gzw := gzip.NewWriter(buf)
	if _, err := gzw.Write(input); err != nil {
		return nil, err
	}
	if err := gzw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
