package ath

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"

	"github.com/andybalholm/brotli"
)

type Compression interface {
	Wrap(io.Writer) io.WriteCloser
	WriteEncodingHeader(http.ResponseWriter)
	AddExtension(string) string
}

type identity struct{}

type nopWriteCloser struct {
	w io.Writer
}

func (w nopWriteCloser) Close() error {
	return nil
}
func (w nopWriteCloser) Write(data []byte) (int, error) {
	return w.w.Write(data)
}

func (i identity) Wrap(w io.Writer) io.WriteCloser {
	return nopWriteCloser{w}
}

func (i identity) WriteEncodingHeader(http.ResponseWriter) {
}

func (i identity) AddExtension(path string) string {
	return path
}

var Identity = identity{}

type compressionFactory func(io.Writer) io.WriteCloser

type compression struct {
	factory compressionFactory
	name    string
	ext     string
}

func (c compression) Wrap(w io.Writer) io.WriteCloser {
	return c.factory(w)
}

func (c compression) WriteEncodingHeader(w http.ResponseWriter) {
	w.Header().Set("Content-Encoding", c.name)
}

func (c compression) AddExtension(path string) string {
	return path + c.ext
}

var GZIP = compression{
	factory: func(w io.Writer) io.WriteCloser { return gzip.NewWriter(w) },
	name:    "gzip",
	ext:     ".gz",
}

var Deflate = compression{
	factory: func(w io.Writer) io.WriteCloser { res, _ := flate.NewWriter(w, -1); return res },
	name:    "deflate",
	ext:     ".deflate",
}

var Brotli = compression{
	factory: func(w io.Writer) io.WriteCloser { return brotli.NewWriter(w) },
	name:    "br",
	ext:     ".br",
}

func CompressAll(compression Compression, r io.Reader) ([]byte, error) {
	buffer := bytes.NewBuffer(nil)
	comp := compression.Wrap(buffer)
	_, err := io.Copy(comp, r)
	if err != nil {
		return nil, err
	}
	err = comp.Close()
	return buffer.Bytes(), err
}
