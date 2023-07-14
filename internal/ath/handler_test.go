package ath

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	. "gopkg.in/check.v1"
)

type HandlerSuite struct {
	logs    *observer.ObservedLogs
	restore func()
}

var _ = Suite(&HandlerSuite{})

func (s *HandlerSuite) SetUpTest(c *C) {
	var core zapcore.Core
	core, s.logs = observer.New(zapcore.InfoLevel)
	log, err := zap.NewProduction(zap.WrapCore(func(zapcore.Core) zapcore.Core {
		return core
	}))
	c.Assert(err, IsNil)
	s.restore = zap.ReplaceGlobals(log)
}

func (s *HandlerSuite) TearDownTest(c *C) {
	s.restore()
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

	logs := s.logs.TakeAll()

	c.Assert(logs, HasLen, 2)
	for _, log := range logs {
		c.Check(log.Level, Equals, zapcore.InfoLevel)
		if c.Check(len(log.Context) >= 4, Equals, true) == false {
			continue
		}
		c.Check(log.Context[0], Equals, zap.String("method", "GET"))
		c.Check(log.Context[1], Equals, zap.String("URL", "/"))
		c.Check(log.Context[2].Key, Equals, "address")
		c.Check(log.Context[3].Key, Equals, "user-agent")
	}

	c.Check(logs[0].Message, Matches, "redirecting to '/index.html'")
	c.Check(logs[1].Message, Matches, "request")
	c.Assert(logs[1].Context, HasLen, 5)
	c.Check(logs[1].Context[4], Equals, zap.Int("status", 404))

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

	logs := s.logs.TakeAll()
	c.Assert(logs, HasLen, 2)
	for _, log := range logs {
		c.Check(log.Level, Equals, zapcore.InfoLevel)
		if c.Check(len(log.Context) >= 4, Equals, true) == false {
			continue
		}
		c.Check(log.Context[0], Equals, zap.String("method", "POST"))
		c.Check(log.Context[1], Equals, zap.String("URL", "/index.html"))
		c.Check(log.Context[2].Key, Equals, "address")
		c.Check(log.Context[3].Key, Equals, "user-agent")
	}

	c.Check(logs[0].Message, Matches, "redirecting to '/index.html'")
	c.Check(logs[1].Message, Matches, "request")
	c.Assert(logs[1].Context, HasLen, 5)
	c.Check(logs[1].Context[4], Equals, zap.Int("status", 404))

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
	logs := s.logs.TakeAll()

	c.Assert(logs, HasLen, 1)
	c.Check(logs[0].Level, Equals, zapcore.InfoLevel)
	c.Check(logs[0].Message, Matches, "request")
	c.Assert(logs[0].Context, HasLen, 5)
	c.Check(logs[0].Context[4], Equals, zap.Int("status", 200))

}
