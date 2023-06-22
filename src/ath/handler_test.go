package ath

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	. "gopkg.in/check.v1"
)

type HandlerSuite struct {
}

var _ = Suite(&HandlerSuite{})

func (s *HandlerSuite) TestInfoLog(c *C) {
	h := NewHandler(nil, false)
	c.Assert(h.info, Not(IsNil))
	c.Check(h.info.Writer(), Equals, io.Discard)

	h = NewHandler(nil, true)
	c.Assert(h.info, Not(IsNil))
	c.Check(h.info.Writer(), Equals, os.Stderr)
}

func (s *HandlerSuite) TestEmptyRouting(c *C) {
	h := NewHandler(nil, true)
	info := bytes.NewBuffer(nil)
	h.info.SetOutput(info)

	w := NewMockResponseWritter()
	req, err := http.NewRequest("GET", "/", bytes.NewBuffer(nil))

	c.Assert(err, IsNil)
	h.ServeHTTP(w, req)

	c.Check(string(w.buffer.Bytes()), ResponseMatches, []string{
		"HTTP/1.1 404 Ok",
		"Content-Type: text/plain; charset=utf-8",
		"X-Content-Type-Options: nosniff",
		"",
		"not found",
	})

	c.Check(string(info.Bytes()), ResponseMatches, `\Q[INFO]\E redirecting to '/index.html'
\Q[INFO]\E GET "/" from .* as ".*": 404`)

}

func (s *HandlerSuite) TestPostMethod(c *C) {
	h := NewHandler(nil, true)
	info := bytes.NewBuffer(nil)
	h.info.SetOutput(info)

	w := NewMockResponseWritter()
	req, err := http.NewRequest("POST", "/index.html", bytes.NewBuffer(nil))

	c.Assert(err, IsNil)
	h.ServeHTTP(w, req)

	c.Check(string(w.buffer.Bytes()), ResponseMatches, []string{
		"HTTP/1.1 404 Ok",
		"Content-Type: text/plain; charset=utf-8",
		"X-Content-Type-Options: nosniff",
		"",
		"not found",
	})

	c.Check(string(info.Bytes()), ResponseMatches, `\Q[INFO]\E POST "/index.html" from .* as ".*": 404`)

}

func (s *HandlerSuite) TestStaticRoute(c *C) {
	dir := c.MkDir()
	c.Assert(ioutil.WriteFile(filepath.Join(dir, "index.html"),
		[]byte(`<html><head/><body/></html>`), 0644), IsNil)
	routes := map[string]Route{
		"/index.html": StaticRoute{
			route:    route{"index.html", "", nil},
			filepath: filepath.Join(dir, "index.html"),
			cache:    NewCache(-1),
		},
	}
	h := NewHandler(routes, true)
	info := bytes.NewBuffer(nil)
	h.info.SetOutput(info)

	w := NewMockResponseWritter()
	req, err := http.NewRequest("GET", "/index.html", bytes.NewBuffer(nil))

	c.Assert(err, IsNil)
	h.ServeHTTP(w, req)

	c.Check(string(w.buffer.Bytes()), ResponseMatches, []string{
		"HTTP/1.1 200 Ok",
		"Accept-Ranges: bytes",
		"Cache-Control: no-store",
		"Content-Length: 27",
		"Content-Type: text/html; charset=utf-8",
		"",
		"<html><head/><body/></html>",
	})

	c.Check(string(info.Bytes()), ResponseMatches, `\Q[INFO]\E GET "/index.html" from .* as ".*": 200`)

}
