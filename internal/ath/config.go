package ath

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ByteSize int64

var prefixes = []string{"", "k", "M", "G", "T"}

func (s ByteSize) MarshalFlag() (string, error) {
	value := float64(s)
	var prefix string
	for _, prefix = range prefixes {
		if math.Abs(value) < 1024 {
			break
		}
		value /= 1024.0
	}
	return fmt.Sprintf("%d%s", int64(value), prefix), nil
}

var byteRx = regexp.MustCompile(`\A([+-]{0,1}[[:digit:]]+)([a-zA-Z]*)\z`)

func (s *ByteSize) UnmarshalFlag(value string) error {
	value = strings.TrimSpace(value)

	m := byteRx.FindAllStringSubmatch(value, -1)
	if len(m) != 1 || len(m[0]) != 3 {
		return fmt.Errorf("invalid format for '%s'", value)
	}

	v, err := strconv.ParseInt(m[0][1], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid number format '%s' in '%s'", m[0][1], value)
	}
	*s = ByteSize(v)
	for _, prefix := range prefixes {
		if prefix == m[0][2] {
			return nil
		}
		*s *= 1024
	}

	*s = 0
	return fmt.Errorf("invalid suffix '%s' in '%s'", m[0][2], value)
}

type Config struct {
	Address string `short:"a" long:"address" description:"address to listen to" default:"0.0.0.0"`
	Port    int    `short:"p" long:"port" description:"port to listen on" default:"80"`
	Verbose []bool `short:"v" long:"verbose" description:"Enable verbose logging for each request"`

	Compression struct {
		NoGZIP    bool     `long:"no-gzip" description:"disable gzip compression"`
		NoDeflate bool     `long:"no-deflate" description:"disable deflate compression"`
		NoBrotli  bool     `long:"no-brotli" description:"disable brotli compression"`
		Eligible  []string `long:"elligible" description:"list of extension to determine files elligible for compression" default:"txt" default:"js" default:"js.map" default:"html" default:"webmanifest" default:"svg" default:"ttf" default:"otf" default:"xml"`
		Threshold ByteSize `long:"threshold" description:"file size threshold to enable compression" default:"1k"`
	} `group:"compression" namespace:"compression"`

	Cache struct {
		MaxAge time.Duration `long:"max-age" description:"Cache-Control max-age on unversionned files" default:"0s"`
	} `group:"cache-control" namespace:"cache"`

	ServerCache struct {
		RootFileInLRU bool     `long:"root-files-in-lru" description:"by default all cacheable root file (non-asset files) are always cached in memory, this option disable it and put it in the LRU cache like other assets"`
		MaxMemorySize ByteSize `short:"m" long:"max-size" description:"maximal size of the cache in bytes" default:"50M"`
	} `group:"server-cache" namespace:"server-cache"`

	CSP struct {
		Disable    bool     `long:"nonce-disable" description:"Disable CSP Nonce generation"`
		NoncedPath []string `short:"O" long:"nonced" description:"list of nonced file" default:"/index.html"`
		Policy     string   `long:"policy" description:"CSP to use" default:"default-src 'self'; style-src 'self' 'nonce-CSP_NONCE'; script-src 'self' 'nonce-CSP_NONCE'"`
	} `group:"csp-nonce" namespace:"csp"`

	Otel struct {
		Endpoint          string `long:"endpoint" description:"Open Telemetry Collectore Endpoint"`
		ServiceName       string `long:"name" description:"Service name to report" default:"angular-to-http"`
		ServiceInstanceID string `long:"instance" description:"Service Instance ID, if empty hostname will be used"`
	} `group:"otel" namespace:"otel"`

	Args struct {
		Directory string `description:"directory to serve (default: '.')" positional-arg-name:"directory"`
	} `positional-args:"yes"`
}

func (c *Config) EnabledCompressions() []Compression {
	//TODO: should not be recomputed but done only once
	res := make([]Compression, 0, 3)
	if c.Compression.NoBrotli == false {
		res = append(res, Brotli)
	}
	if c.Compression.NoGZIP == false {
		res = append(res, GZIP)
	}
	if c.Compression.NoDeflate == false {
		res = append(res, Deflate)
	}
	return res
}

func (c *Config) AllowedCompressions() map[string]bool {
	res := make(map[string]bool)
	for _, ext := range c.Compression.Eligible {
		res["."+ext] = true
	}
	return res
}
