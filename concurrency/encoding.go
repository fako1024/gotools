package concurrency

import (
	"compress/gzip"
	"errors"
	"io"
	"sync"

	jsoniter "github.com/json-iterator/go"
	yaml "gopkg.in/yaml.v3"
)

var (
	gzipWPool = sync.Pool{
		New: func() any {
			return &gzip.Writer{}
		},
	}
	gzipRPool = sync.Pool{
		New: func() any {
			return &gzip.Reader{}
		},
	}
)

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
)

// Writer denotes a generic writer interface (enforcing an initialization and closing method)
type Writer interface {
	Init(w io.Writer) io.Writer
	Close()
}

// Reader denotes a generic reader interface (enforcing an initialization and closing method)
type Reader interface {
	Init(r io.Reader) (io.Reader, error)
	Close()
}

// GZIPWriter provides a wrapper around a standard gzip.Writer instance
type GZIPWriter struct {
	*gzip.Writer
}

// NewGZIPWriter initializes a new (wrapped) gzip.Writer instance, fulfilling the Writer interface
func NewGZIPWriter() *GZIPWriter {
	return &GZIPWriter{
		Writer: gzipWPool.Get().(*gzip.Writer),
	}
}

// Init resets a (wrapped) gzip.Writer instance from the pool for reuse
func (g *GZIPWriter) Init(w io.Writer) io.Writer {
	g.Reset(w)
	return g.Writer
}

// Close returns a (wrapped) gzip.Writer instance to the pool
func (g *GZIPWriter) Close() {
	gzipWPool.Put(g.Writer)
}

// GZIPReader provides a wrapper around a standard gzip.Reader instance
type GZIPReader struct {
	*gzip.Reader
}

// NewGZIPReader initializes a new (wrapped) gzip.Reader instance, fulfilling the Reader interface
func NewGZIPReader() *GZIPReader {
	return &GZIPReader{
		Reader: gzipRPool.Get().(*gzip.Reader),
	}
}

// Init resets a (wrapped) gzip.Reader instance from the pool for reuse
func (g *GZIPReader) Init(r io.Reader) (io.Reader, error) {
	err := g.Reset(r)
	return g.Reader, err
}

// Close returns a (wrapped) gzip.Reader instance to the pool
func (g *GZIPReader) Close() {
	gzipRPool.Put(g.Reader)
}

// Encoder denotes a generic encoder interface
type Encoder interface {
	Encode(v any) error
}

// Decoder denotes a generic decoder interface
type Decoder interface {
	Decode(v any) error
}

// ReaderFn denotes a chained io.Reader based function / method
type ReaderFn func(r io.Reader) (io.Reader, error)

// DecoderFn denotes an io.Reader based decoder function / method
type DecoderFn func(r io.Reader) Decoder

// WriterFn denotes a chained io.Writer based function / method
type WriterFn func(w io.Writer) io.Writer

// EncoderFn denotes an io.Writer based encoder function / method
type EncoderFn func(w io.Writer) Encoder

// WriterChain provides convenient access to a chained io.Writer sequence (and potentially encoding)
type WriterChain struct {
	writers []Writer
	closers []io.Closer

	postFn  func(rw *ReadWriter) error
	dest    *ReadWriter
	memPool *MemPoolNoLimit

	io.Writer
}

// NewWriterChain instantiates a new WriterChain
func NewWriterChain() *WriterChain {
	return &WriterChain{
		memPool: defaultMemPool,
		writers: make([]Writer, 0),
	}
}

// AddWriter adds a Writer instance to the chain
func (wc *WriterChain) AddWriter(w Writer) *WriterChain {
	wc.writers = append(wc.writers, w)
	return wc
}

// MemPool sets an (external) memory pool for the chain of Writers
func (wc *WriterChain) MemPool(memPool *MemPoolNoLimit) *WriterChain {
	wc.memPool = memPool
	return wc
}

// PostFn sets a function to be executed at the end of the Writer / encoding chain
func (wc *WriterChain) PostFn(fn func(rw *ReadWriter) error) *WriterChain {
	wc.postFn = fn
	return wc
}

// Build constructs the chain of Writers and defines / defers potential cleanup function calls
func (wc *WriterChain) Build() *WriterChain {

	var w io.Writer
	wc.dest = wc.memPool.GetReadWriter(0)
	w = wc.dest

	for _, writer := range wc.writers {
		addW := writer.Init(w)
		if addWCloser, ok := addW.(io.Closer); ok {
			wc.closers = append(wc.closers, addWCloser)
		}
		w = addW
	}

	wc.Writer = w
	return wc
}

// Close closes the Writer chain, flushing all underlying Writers
func (wc *WriterChain) Close() error {
	defer wc.memPool.PutReadWriter(wc.dest)

	for i := len(wc.closers) - 1; i >= 0; i-- {
		if err := wc.closers[i].Close(); err != nil {
			return err
		}
	}
	if wc.postFn != nil {
		return wc.postFn(wc.dest)
	}
	for _, writer := range wc.writers {
		writer.Close()
	}

	return nil
}

// Encode encodes the output of the chain of Writers into an object using the provided encoder function
func (wc *WriterChain) Encode(fn EncoderFn, v any) (*ReadWriter, error) {
	if fn == nil {
		return nil, errors.New("nil encoder function")
	}
	err := fn(wc.Writer).Encode(v)
	return wc.dest, err
}

// EncodeAndClose performs the encoding and closes / flushes all Writers in the chain simultaneously
func (wc *WriterChain) EncodeAndClose(fn EncoderFn, v any) error {
	if _, err := wc.Encode(fn, v); err != nil {
		return err
	}
	return wc.Close()
}

// ReaderChain provides convenient access to a chained io.Reader sequence (and potentially decoding)
type ReaderChain struct {
	readers  []Reader
	closers  []io.Closer
	buildErr error

	postFn  func(rw *ReadWriter) error
	dest    *ReadWriter
	memPool *MemPoolNoLimit

	io.Reader
}

// NewReaderChain instantiates a new ReaderChain
func NewReaderChain(r io.Reader) *ReaderChain {
	return &ReaderChain{
		Reader:  r,
		memPool: defaultMemPool,
		readers: make([]Reader, 0),
	}
}

// AddReader adds a Reader instance to the chain
func (rc *ReaderChain) AddReader(r Reader) *ReaderChain {
	rc.readers = append(rc.readers, r)
	return rc
}

// MemPool sets an (external) memory pool for the chain of Readers
func (rc *ReaderChain) MemPool(memPool *MemPoolNoLimit) *ReaderChain {
	rc.memPool = memPool
	return rc
}

// PostFn sets a function to be executed at the end of the Reader / decoding chain
func (rc *ReaderChain) PostFn(fn func(rw *ReadWriter) error) *ReaderChain {
	rc.postFn = fn
	return rc
}

// Build constructs the chain of Readers and defines / defers potential cleanup function calls
func (rc *ReaderChain) Build() *ReaderChain {
	r := rc.Reader
	if rCloser, ok := r.(io.Closer); ok {
		rc.closers = append(rc.closers, rCloser)
	}

	for _, reader := range rc.readers {
		addR, err := reader.Init(r)
		if err != nil {
			rc.buildErr = err
			return rc
		}
		if addRCloser, ok := addR.(io.Closer); ok {
			rc.closers = append(rc.closers, addRCloser)
		}
		r = addR
	}

	rc.Reader = r
	return rc
}

// Close closes the Reader chain, flushing all underlying Readers
func (rc *ReaderChain) Close() error {
	for i := len(rc.closers) - 1; i >= 0; i-- {
		if err := rc.closers[i].Close(); err != nil {
			return err
		}
	}
	if rc.postFn != nil {
		return rc.postFn(rc.dest)
	}
	for _, reader := range rc.readers {
		reader.Close()
	}
	return nil
}

// Decode decodes from an object using the provided decoder function
func (rc *ReaderChain) Decode(fn DecoderFn, v any) error {
	if rc.buildErr != nil {
		return rc.buildErr
	}
	if fn == nil {
		return errors.New("nil decoder function")
	}
	return fn(rc.Reader).Decode(v)
}

// DecodeAndClose performs the decoding and closes / flushes all Readers in the chain simultaneously
func (rc *ReaderChain) DecodeAndClose(fn DecoderFn, v any) error {
	if err := rc.Decode(fn, v); err != nil {
		return err
	}
	return rc.Close()
}
