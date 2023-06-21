package ath

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"errors"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/andybalholm/brotli"
	. "gopkg.in/check.v1"
)

type CompressionSuite struct {
	testdata map[string]Compression
}

var _ = Suite(&CompressionSuite{})

func (s *CompressionSuite) SetUpSuite(c *C) {
	s.testdata = map[string]Compression{
		"Identity": Identity,
		"GZIP":     GZIP,
		"Brotli":   Brotli,
		"Deflate":  Deflate,
	}
}

var decompressFactory = map[string]func(io.Reader) io.ReadCloser{
	"Identity": io.NopCloser,
	"GZIP": func(r io.Reader) io.ReadCloser {
		res, _ := gzip.NewReader(r)
		return res
	},
	"Deflate": flate.NewReader,
	"Brotli": func(r io.Reader) io.ReadCloser {
		return io.NopCloser(brotli.NewReader(r))
	},
}

func decompress(r io.Reader, name string) (string, error) {
	factory, ok := decompressFactory[name]
	if ok == false {
		return "", errors.New("no factory")
	}
	decomp := factory(r)
	defer decomp.Close()
	res, err := ioutil.ReadAll(decomp)
	return string(res), err
}

func (s *CompressionSuite) TestCompressAll(c *C) {
	input := "Hello, World!"
	for name, compression := range s.testdata {
		comment := Commentf("compression %s", name)
		res, err := CompressAll(compression, bytes.NewReader([]byte(input)))
		if c.Check(err, IsNil, comment) == false {
			continue
		}
		output, err := decompress(bytes.NewReader(res), name)
		if c.Check(err, IsNil, comment) == false {
			continue
		}
		c.Check(output, Equals, input)
	}
}

func (s *CompressionSuite) TestAddExtension(c *C) {
	testdata := []struct {
		Name, File, Expected string
	}{
		{"Identity", "index.html", "index.html"},
		{"GZIP", "script.js", "script.js.gz"},
		{"Deflate", "font.ttf", "font.ttf.deflate"},
		{"Brotli", "index.html", "index.html.br"},
	}

	for _, d := range testdata {
		comment := Commentf("Compression %s", d.Name)
		comp, ok := s.testdata[d.Name]
		if c.Check(ok, Equals, true, comment) == false {
			continue
		}
		c.Check(comp.AddExtension(d.File), Equals, d.Expected)
	}
}

type mockResponseWriter struct {
	header http.Header
}

func (w *mockResponseWriter) Header() http.Header {
	return w.header
}

func (w *mockResponseWriter) WriteHeader(int) {
}

func (w *mockResponseWriter) Write(data []byte) (int, error) {
	return len(data), nil
}

func (s *CompressionSuite) TestWriteHeader(c *C) {
	testdata := []struct {
		Name, Expected string
	}{
		{"Identity", ""},
		{"GZIP", "Content-Encoding: gzip\r\n"},
		{"Deflate", "Content-Encoding: deflate\r\n"},
		{"Brotli", "Content-Encoding: br\r\n"},
	}

	for _, d := range testdata {
		comment := Commentf("Compression %s", d.Name)
		comp, ok := s.testdata[d.Name]
		if c.Check(ok, Equals, true, comment) == false {
			continue
		}
		w := &mockResponseWriter{header: make(http.Header)}
		comp.WriteEncodingHeader(w)
		buf := bytes.NewBuffer(nil)
		w.header.Write(buf)
		c.Check(string(buf.Bytes()), Equals, d.Expected)

	}
}
