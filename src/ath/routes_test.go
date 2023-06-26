package ath

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"golang.org/x/exp/constraints"
	. "gopkg.in/check.v1"
)

type responseChecker struct {
	*CheckerInfo
}

var ResponseMatches = &responseChecker{
	&CheckerInfo{Name: "ResponseMatches", Params: []string{"value", "regexes"}},
}

func (checker *responseChecker) Check(params []interface{}, names []string) (result bool, error string) {
	var value string
	switch v := params[0].(type) {
	case string:
		value = v
	case []byte:
		value = string(v)
	case fmt.Stringer:
		value = v.String()
	default:
		return false, "Value should be []byte, string or Implement fmt.Stringer"
	}

	var regexes []string
	switch v := params[1].(type) {
	case string:
		regexes = []string{v}
	case []byte:
		regexes = []string{string(v)}
	case []string:
		regexes = v
	case [][]byte:
		regexes = make([]string, len(v))
		for i, vv := range v {
			regexes[i] = string(vv)
		}
	default:
		return false, "Regexes should be []string or string"
	}
	regex := strings.Join(regexes, "\r\n")
	rx, err := regexp.Compile(regex)
	if err != nil {
		return false, "Can´t compile regex: " + err.Error()
	}
	return rx.MatchString(value), ""
}

type RoutesSuite struct {
	dir      string
	filepath string
	content  string
}

var _ = Suite(&RoutesSuite{})

func (s *RoutesSuite) SetUpSuite(c *C) {
	s.dir = c.MkDir()
	s.content = `<html>
  <head>
  </head>
  <body>
    <h1>Hello World!</h1>
  </body>
</html>`
	s.filepath = filepath.Join(s.dir, "index.html")
	c.Assert(ioutil.WriteFile(s.filepath, []byte(s.content), 0644), IsNil)
}

func (s *RoutesSuite) TestRouteFlags(c *C) {
	testdata := []struct {
		Flags    RouteFlag
		Expected string
	}{
		{COMPRESSIBLE, "COMPRESSIBLE"},
		{IMMUTABLE, "IMMUTABLE"},
		{NONCED, "NONCED"},
		{NONCED | COMPRESSIBLE, "COMPRESSIBLE, NONCED"},
		{IMMUTABLE | COMPRESSIBLE, "COMPRESSIBLE, IMMUTABLE"},
	}

	for _, d := range testdata {
		c.Check(d.Flags.String(), Equals, d.Expected, Commentf("%v", d))
	}
}

func (s *RoutesSuite) TestRouteRouteFlags(c *C) {
	testdata := []struct {
		Route Route
		Flags RouteFlag
	}{
		{
			StaticRoute{
				route: route{"index.html", "text/html; charset:utf-8", nil},
			},
			0,
		},
		{
			StaticRoute{
				route: route{"index.html", "text/html; charset:utf-8", []Compression{GZIP}},
			},
			COMPRESSIBLE,
		},
		{
			StaticRoute{
				route:        route{"index.html", "text/html; charset:utf-8", nil},
				cacheControl: "max-age=10; immutable"},
			IMMUTABLE,
		},
		{
			StaticRoute{
				route:        route{"index.html", "text/html; charset:utf-8", []Compression{GZIP}},
				cacheControl: "max-age=10; immutable"},
			COMPRESSIBLE | IMMUTABLE,
		},
		{
			NoncedRoute{
				route: route{"index.html", "text/html; charset:utf-8", []Compression{GZIP}},
			},
			COMPRESSIBLE | NONCED,
		},
		{
			NoncedRoute{
				route: route{"index.html", "text/html; charset=utf-8", nil},
			},
			NONCED,
		},
	}

	for _, d := range testdata {
		c.Check(d.Route.Flags(), Equals, d.Flags)
	}

}

func (s *RoutesSuite) TestStaticRoutePreCache(c *C) {
	testdata := []struct {
		Route StaticRoute
		Keys  []string
	}{
		{
			Route: StaticRoute{
				route:    route{"index.html", "text/html; charset: utf-8", nil},
				filepath: s.filepath,
			},
			Keys: []string{s.filepath},
		},
		{
			Route: StaticRoute{
				route: route{"index.html", "text/html; charset: utf-8",
					[]Compression{GZIP, Brotli, Deflate}},
				filepath: s.filepath,
			},
			Keys: []string{
				s.filepath,
				s.filepath + ".gz",
				s.filepath + ".br",
				s.filepath + ".deflate",
			},
		},
	}

	for _, d := range testdata {
		cache := NewCache(-1)
		d.Route.cache = cache
		d.Route.PreCache()
		for _, key := range d.Keys {
			c.Check(hasKey(cache, key), Equals, true, Commentf("key: %s for %v", key, d.Route))
		}
	}

}

func Min[T constraints.Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}

func (s *RoutesSuite) TestStaticRouteServeHTTP(c *C) {
	t := time.Date(2023, 04, 01, 0, 0, 0, 0, time.Local).In(time.FixedZone("GMT", 0))
	testdata := []struct {
		Route          StaticRoute
		AcceptEncoding string
		Content        []string
	}{
		{
			Route: StaticRoute{
				route:        route{"index.html", "text/html; charset=utf-8", nil},
				filepath:     s.filepath,
				cacheControl: "no-cache",
			},
			Content: []string{
				"HTTP/1.1 200 Ok",
				"Accept-Ranges: bytes",
				"Cache-Control: no-cache",
				"Content-Length: 78",
				"Content-Type: text/html; charset=utf-8",
				"Last-Modified: " + t.Format(time.RFC1123),
				"",
				s.content + `\z`,
			},
		},
		{
			Route: StaticRoute{
				route:        route{"index.html", "text/html; charset=utf-8", nil},
				cacheControl: "max-age=300",
				filepath:     s.filepath,
			},
			Content: []string{
				"HTTP/1.1 200 Ok",
				"Accept-Ranges: bytes",
				"Cache-Control: max-age=300",
				"Content-Length: 78",
				"Content-Type: text/html; charset=utf-8",
				"Last-Modified: " + t.Format(time.RFC1123),
				"",
				s.content + `\z`,
			},
		},
		{
			Route: StaticRoute{
				route:    route{"index.html", "text/html; charset=utf-8", []Compression{GZIP, Brotli}},
				filepath: s.filepath,
			},
			AcceptEncoding: "*",
			Content: []string{
				"HTTP/1.1 200 Ok",
				"Accept-Ranges: bytes",
				"Content-Encoding: gzip",
				"Content-Type: text/html; charset=utf-8",
				"Last-Modified: " + t.Format(time.RFC1123),
				"",
			},
		},
		{
			Route: StaticRoute{
				route:    route{"index.html", "text/html; charset=utf-8", []Compression{GZIP, Brotli}},
				filepath: s.filepath,
			},
			AcceptEncoding: "br",
			Content: []string{
				"HTTP/1.1 200 Ok",
				"Accept-Ranges: bytes",
				"Content-Encoding: br",
				"Content-Type: text/html; charset=utf-8",
				"Last-Modified: " + t.Format(time.RFC1123),
				"",
			},
		},
	}
	cache := NewCache(1)
	for _, d := range testdata {

		d.Route.cache = cache
		d.Route.modtime = t
		w := NewMockResponseWritter()
		req, err := http.NewRequest("GET", "/index.html", bytes.NewBuffer(nil))
		if len(d.AcceptEncoding) > 0 {
			req.Header.Add("Accept-Encoding", d.AcceptEncoding)
		}
		if c.Check(err, IsNil) == false {
			continue
		}
		d.Route.ServeHTTP(w, req)
		c.Check(string(w.buffer.Bytes()), ResponseMatches, d.Content)
	}

}

func (s *RoutesSuite) TestStaticRouteFailure(c *C) {
	r := StaticRoute{route: route{"index.html", "text/html; charset=utf-8", nil},
		filepath: filepath.Join(s.dir, "do-not-exists"),
		cache:    NewCache(-1),
	}
	req, err := http.NewRequest("GET", "/index.html", bytes.NewBuffer(nil))
	c.Assert(err, IsNil)
	w := NewMockResponseWritter()
	logBuffer := bytes.NewBuffer(nil)
	log.SetOutput(logBuffer)
	r.ServeHTTP(w, req)
	c.Check(string(w.buffer.Bytes()), Equals, "HTTP/1.1 500 Ok\r\n"+
		"Content-Type: text/plain; charset=utf-8\r\n"+
		"X-Content-Type-Options: nosniff\r\n"+
		"\r\n"+
		"read error\n")
	rx := regexp.MustCompile(`\Q[WARNING]\E open .*/do-not-exists: no such file or directory`)
	c.Check(rx.Match(logBuffer.Bytes()), Equals, true)

}

func (s *RoutesSuite) TestNoncedRoute(c *C) {
	tmpl := template.Must(template.New("CSP").Parse(`default-src 'self'; style-src 'self' 'nonce-{{.Nonce}}'; script-src 'self' 'nonce-{{.Nonce}}'`))

	tmpl = template.Must(tmpl.New("content").Parse(`<html><head><script nonce="{{.Nonce}}"></script></head><body></body></html>`))

	r := NoncedRoute{
		route:    route{"index.html", "text/html; charset=utf-8", nil},
		template: tmpl,
	}

	w := NewMockResponseWritter()
	req, err := http.NewRequest("GET", "/index.html", bytes.NewBuffer(nil))
	c.Assert(err, IsNil)
	r.ServeHTTP(w, req)
	rx := regexp.MustCompile("HTTP/1.1 200 Ok\r\n" +
		"Accept-Ranges: bytes\r\n" +
		"Cache-Control: no-store\r\n" +
		"Content-Length: [0-9]+\r\n" +
		"Content-Security-Policy: default-src 'self'; style-src 'self' 'nonce-(.*)'; script-src 'self' 'nonce-(.*)'\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"Last-Modified: .* GMT\r\n\r\n" +
		"<html><head><script nonce=\"(.*)\"></script></head><body></body></html>")

	c.Log(string(w.buffer.Bytes()))
	m := rx.FindAllStringSubmatch(string(w.buffer.Bytes()), -1)
	c.Assert(len(m), Equals, 1)
	c.Assert(len(m[0]), Equals, 4)
	c.Check(m[0][1], Equals, m[0][2])
	c.Check(m[0][1], Equals, m[0][3])

}

func (s *RoutesSuite) TestNoncedRouteFailure(c *C) {
	tmpl := template.Must(template.New("CSP").Parse(`default-src 'self'; style-src 'self' 'nonce-{{.N}}'; script-src 'self' 'nonce-{{.Nonce}}'`))

	tmpl = template.Must(tmpl.New("content").Parse(`<html><head><script nonce="{{.N}}"></script></head><body></body></html>`))

	r := NoncedRoute{
		route:    route{"index.html", "text/html; charset=utf-8", nil},
		template: tmpl,
	}
	req, err := http.NewRequest("GET", "/index.html", bytes.NewBuffer(nil))
	w := NewMockResponseWritter()
	c.Assert(err, IsNil)
	logBuffer := bytes.NewBuffer(nil)
	log.SetOutput(logBuffer)

	r.ServeHTTP(w, req)
	c.Check(w.buffer.Bytes(), ResponseMatches, []string{"HTTP/1.1 500 Ok",
		"Content-Type: text/plain; charset=utf-8",
		"X-Content-Type-Options: nosniff",
		"",
		"internal server error"})

	c.Check(logBuffer.Bytes(), ResponseMatches, `\Q[WARNING]\E could not execute response template for index\.html: template: content:.*: executing "content" at <\.N>`)

	tmpl = template.Must(tmpl.New("content").Parse(`<html><head><script nonce="{{.Nonce}}"></script></head><body></body></html>`))
	r.template = tmpl

	w = NewMockResponseWritter()
	logBuffer = bytes.NewBuffer(nil)
	log.SetOutput(logBuffer)

	r.ServeHTTP(w, req)
	c.Check(w.buffer.Bytes(), ResponseMatches, []string{"HTTP/1.1 500 Ok",
		"Content-Type: text/plain; charset=utf-8",
		"X-Content-Type-Options: nosniff",
		"",
		"internal server error"})

	c.Check(logBuffer.Bytes(), ResponseMatches, `\Q[WARNING]\E could not execute CSP template for index\.html: template: CSP:.*: executing "CSP" at <\.N>`)

}
