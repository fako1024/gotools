package concurrency

import (
	"compress/gzip"
	"io"

	jsoniter "github.com/json-iterator/go"
	yaml "gopkg.in/yaml.v3"
)

type Encoder interface {
	Encode(v any) error
}

type Decoder interface {
	Decode(v any) error
}

type ReaderFn func(r io.Reader) (io.Reader, error)
type DecoderFn func(r io.Reader) Decoder

type WriterFn func(w io.Writer) io.Writer
type EncoderFn func(w io.Writer) Encoder

type EncoderChain struct {
	writer    io.Writer
	encoderFn EncoderFn
	writerFns []WriterFn

	closers []io.Closer
}

type DecoderChain struct {
	reader    io.Reader
	decoderFn DecoderFn
	readerFns []ReaderFn

	buildErr error
	closers  []io.Closer
}

func NewDecoderChain(r io.Reader, fn DecoderFn) *DecoderChain {
	return &DecoderChain{
		reader:    r,
		decoderFn: fn,
		readerFns: make([]ReaderFn, 0),
	}
}

func NewEncoderChain(w io.Writer, fn EncoderFn) *EncoderChain {
	return &EncoderChain{
		writer:    w,
		encoderFn: fn,
		writerFns: make([]WriterFn, 0),
	}
}

func (ec *EncoderChain) AddWriter(fn WriterFn) *EncoderChain {
	ec.writerFns = append(ec.writerFns, fn)
	return ec
}

func (ec *EncoderChain) Build() *EncoderChain {
	w := ec.writer
	if wCloser, ok := w.(io.Closer); ok {
		ec.closers = append(ec.closers, wCloser)
	}

	for _, fn := range ec.writerFns {
		addW := fn(w)
		if addWCloser, ok := addW.(io.Closer); ok {
			ec.closers = append(ec.closers, addWCloser)
		}
		w = addW
	}

	ec.writer = w
	return ec
}

// TODO: Add convenience Shizzl for Encoders / Decoders

var (
	JSONEncoder = func(w io.Writer) Encoder {
		return jsoniter.NewEncoder(w)
	}
	JSONDecoder = func(r io.Reader) Decoder {
		return jsoniter.NewDecoder(r)
	}
	YAMLEncoder = func(w io.Writer) Encoder {
		return yaml.NewEncoder(w)
	}
	YAMLDecoder = func(r io.Reader) Decoder {
		return yaml.NewDecoder(r)
	}
	GZIPWriter = func(w io.Writer) io.Writer {
		return gzip.NewWriter(w)
	}
	GZIPReader = func(r io.Reader) (io.Reader, error) {
		return gzip.NewReader(r)
	}
)

func (ec *EncoderChain) Encode(v any) error {
	if ec.encoderFn == nil {
		return nil
	}
	return ec.encoderFn(ec.writer).Encode(v)
}

func (ec *EncoderChain) Close() error {
	for i := len(ec.closers) - 1; i >= 0; i-- {
		if err := ec.closers[i].Close(); err != nil {
			return err
		}
	}
	return nil
}

func (ec *EncoderChain) EncodeAndClose(v any) error {
	if err := ec.Encode(v); err != nil {
		return err
	}
	return ec.Close()
}

func (dc *DecoderChain) AddReader(fn ReaderFn) *DecoderChain {
	dc.readerFns = append(dc.readerFns, fn)
	return dc
}

func (dc *DecoderChain) Build() *DecoderChain {
	r := dc.reader
	if rCloser, ok := r.(io.Closer); ok {
		dc.closers = append(dc.closers, rCloser)
	}

	for _, fn := range dc.readerFns {
		addR, err := fn(r)
		if err != nil {
			dc.buildErr = err
			return dc
		}
		if addRCloser, ok := addR.(io.Closer); ok {
			dc.closers = append(dc.closers, addRCloser)
		}
		r = addR
	}

	dc.reader = r
	return dc
}

func (dc *DecoderChain) Decode(v any) error {
	if dc.buildErr != nil {
		return dc.buildErr
	}
	if dc.decoderFn == nil {
		return nil
	}
	return dc.decoderFn(dc.reader).Decode(v)
}

func (dc *DecoderChain) Close() error {
	for i := len(dc.closers) - 1; i >= 0; i-- {
		if err := dc.closers[i].Close(); err != nil {
			return err
		}
	}
	return nil
}

func (dc *DecoderChain) DecodeAndClose(v any) error {
	if err := dc.Decode(v); err != nil {
		return err
	}
	return dc.Close()
}
