package ath

import (
	"bytes"
	"net/http"

	"github.com/jessevdk/go-flags"
	. "gopkg.in/check.v1"
)

type BuildRoutesSuite struct{}

var _ = Suite(&BuildRoutesSuite{})

func checkRoutes(c *C, routes map[string]Route, flags map[string]RouteFlag) {
	for target, flags := range flags {
		route, ok := routes[target]
		if c.Check(ok, Equals, true, Commentf("missing route '%s'", target)) == false {
			continue
		}

		c.Check(route.Flags(), Equals, flags, Commentf("for route '%s'", target))
	}

	for target := range routes {
		_, ok := flags[target]
		c.Check(ok, Equals, true, Commentf("unexpected route '%s'", target))
	}
}

func (s *BuildRoutesSuite) TestDefaultApplication(c *C) {
	var config Config
	_, err := flags.ParseArgs(&config, []string{"utest-data/utest-app", "--compression.threshold=512"})
	c.Assert(err, IsNil)
	routes, err := BuildRoutes(config)
	c.Assert(err, IsNil)
	checkRoutes(c, routes, map[string]RouteFlag{
		"/index.html":                    COMPRESSIBLE,
		"/3rdpartylicenses.txt":          COMPRESSIBLE,
		"/main.d9c155841b368d1f.js":      COMPRESSIBLE | IMMUTABLE,
		"/polyfills.3f5925aa1897dcef.js": COMPRESSIBLE | IMMUTABLE,
		"/favicon.ico":                   0,
		"/assets/random.svg":             COMPRESSIBLE,
		"/runtime.5ba494be3870c376.js":   COMPRESSIBLE | IMMUTABLE,
		"/styles.ef46db3751d8e999.css":   IMMUTABLE,
	})

}

func (s *BuildRoutesSuite) TestNoncedApplication(c *C) {
	var config Config
	_, err := flags.ParseArgs(&config, []string{"utest-data/utest-app-nonced", "--compression.threshold=512"})
	c.Assert(err, IsNil)
	routes, err := BuildRoutes(config)
	c.Assert(err, IsNil)
	checkRoutes(c, routes, map[string]RouteFlag{
		"/index.html":                    COMPRESSIBLE | NONCED,
		"/3rdpartylicenses.txt":          COMPRESSIBLE,
		"/main.d9c155841b368d1f.js":      COMPRESSIBLE | IMMUTABLE,
		"/polyfills.3f5925aa1897dcef.js": COMPRESSIBLE | IMMUTABLE,
		"/favicon.ico":                   0,
		"/assets/random.svg":             COMPRESSIBLE,
		"/runtime.5ba494be3870c376.js":   COMPRESSIBLE | IMMUTABLE,
		"/styles.ef46db3751d8e999.css":   IMMUTABLE,
	})

	index, ok := routes["/index.html"]
	c.Assert(ok, Equals, true)

	w := NewMockResponseWritter()
	req, err := http.NewRequest("GET", "/", bytes.NewBuffer(nil))
	c.Assert(err, IsNil)

	index.ServeHTTP(w, req)

	c.Check(string(w.buffer.Bytes()), ResponseMatches, []string{
		"HTTP/1.1 200 Ok",
		"Accept-Ranges: bytes",
		"Cache-Control: no-store",
		"Content-Length: [0-9]+",
		"Content-Security-Policy: default-src 'self'; style-src 'self' 'nonce-.*'; script-src 'self' 'nonce-.*'",
		"Content-Type: text/html; charset=utf-8",
		"Last-Modified: .*GMT",
		"",
		`.*html.\n<html.*>\n<head>\n  <meta.*\n  <title.*\n  <base .*\n  <meta .*\n  <link .*\n<link .*\n<body>\n  <app-root ngCspNonce="[^"]+"></app-root>`,
	})

}
