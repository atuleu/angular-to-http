package ath

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	. "gopkg.in/check.v1"
)

type HandlerSuite struct {
	hook *test.Hook
}

var _ = Suite(&HandlerSuite{})

func (s *HandlerSuite) SetUpSuite(c *C) {
	_, s.hook = test.NewNullLogger()
	logrus.AddHook(s.hook)
}

func (s *HandlerSuite) SetUpTest(c *C) {
	s.hook.Reset()
	logrus.SetLevel(logrus.InfoLevel)
}

func (s *HandlerSuite) TearDownTest(c *C) {
	s.hook.Reset()
	logrus.SetLevel(logrus.WarnLevel)
}

func (s *HandlerSuite) TestEmptyRouting(c *C) {
	h := NewHandler(nil)

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

	c.Assert(s.hook.Entries, HasLen, 2)
	c.Check(s.hook.Entries[0].Level, Equals, logrus.InfoLevel)
	c.Check(s.hook.Entries[0].Message, Matches, "redirecting to '/index.html'")
	c.Check(s.hook.Entries[1].Level, Equals, logrus.InfoLevel)
	c.Check(s.hook.Entries[1].Message, Matches, "request")

}

func (s *HandlerSuite) TestPostMethod(c *C) {
	h := NewHandler(nil)

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

	c.Assert(s.hook.Entries, HasLen, 2)

	c.Check(s.hook.Entries[0].Level, Equals, logrus.InfoLevel)
	c.Check(s.hook.Entries[0].Message, Matches, "redirecting to '/index.html'")
	c.Check(s.hook.Entries[1].Level, Equals, logrus.InfoLevel)
	c.Check(s.hook.Entries[1].Message, Matches, "request")
	c.Check(s.hook.Entries[1].Data["status"], Equals, 404)

}

func (s *HandlerSuite) TestStaticRoute(c *C) {
	dir := c.MkDir()
	c.Assert(ioutil.WriteFile(filepath.Join(dir, "index.html"),
		[]byte(`<html><head/><body/></html>`), 0644), IsNil)
	routes := map[string]Route{
		"/index.html": StaticRoute{
			route:        route{"index.html", "", nil},
			filepath:     filepath.Join(dir, "index.html"),
			cacheControl: "no-cache",
			cache:        NewCache(-1),
		},
	}
	h := NewHandler(routes)

	w := NewMockResponseWritter()
	req, err := http.NewRequest("GET", "/index.html", bytes.NewBuffer(nil))

	c.Assert(err, IsNil)
	h.ServeHTTP(w, req)

	c.Check(string(w.buffer.Bytes()), ResponseMatches, []string{
		"HTTP/1.1 200 Ok",
		"Accept-Ranges: bytes",
		"Cache-Control: no-cache",
		"Content-Length: 27",
		"Content-Type: text/html; charset=utf-8",
		"",
		"<html><head/><body/></html>",
	})

	c.Assert(s.hook.Entries, HasLen, 1)
	c.Check(s.hook.Entries[0].Level, Equals, logrus.InfoLevel)
	c.Check(s.hook.Entries[0].Message, Matches, "request")
	c.Check(s.hook.Entries[0].Data["status"], Equals, 200)

}
