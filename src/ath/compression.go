package ath

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/andybalholm/brotli"
)

type Compression interface {
	Wrap(io.Writer) io.Writer
	Compress(io.Reader) ([]byte, error)
	WriteEncodingHeader(http.ResponseWriter)
	AddExtension(string) string
}

type identity struct{}

func (i identity) Wrap(w io.Writer) io.Writer {
	return w
}

func (i identity) Compress(r io.Reader) ([]byte, error) {
	return ioutil.ReadAll(r)
}

func (i identity) WriteEncodingHeader(http.ResponseWriter) {
}

func (i identity) AddExtension(path string) string {
	return path
}

var Identity = identity{}

type compressionFactory func(io.Writer) io.Writer

type compression struct {
	factory compressionFactory
	name    string
	ext     string
}

func (c compression) Wrap(w io.Writer) io.Writer {
	return c.factory(w)
}

func (c compression) Compress(r io.Reader) ([]byte, error) {
	buffer := bytes.NewBuffer(nil)
	comp := c.factory(buffer)
	_, err := io.Copy(comp, r)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func (c compression) WriteEncodingHeader(w http.ResponseWriter) {
	w.Header().Set("Content-Encoding", c.name)
}

func (c compression) AddExtension(path string) string {
	return path + c.ext
}

var GZIP = compression{
	factory: func(w io.Writer) io.Writer { return gzip.NewWriter(w) },
	name:    "gzip",
	ext:     ".gz",
}

var Deflate = compression{
	factory: func(w io.Writer) io.Writer { res, _ := flate.NewWriter(w, -1); return res },
	name:    "deflate",
	ext:     ".deflate",
}

var Brotli = compression{
	factory: func(w io.Writer) io.Writer { return brotli.NewWriter(w) },
	name:    "br",
	ext:     ".br",
}
