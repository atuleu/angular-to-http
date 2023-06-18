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

	Compression struct {
		NoGZIP               bool     `long:"no-gzip" description:"disable gzip compression"`
		NoDeflate            bool     `long:"no-deflate" description:"disable deflate compression"`
		NoBrotli             bool     `long:"no-brotli" description:"disable brotli compression"`
		CompressionThreshold ByteSize `long:"threshold" description:"file size threshold to enable compression" default:"1k"`
	} `group:"compression" namespace:"comp"`

	CacheControl struct {
		MaxAge         time.Duration `long:"max-age" description:"cache max age on cacheable file, i.e. files with an hash" default:"168h" default-mask:"1 week"`
		Ignore         []string      `short:"N" long:"no-store" description:"additional files to set Cache-Control: no-store"`
		MaxSize        ByteSize      `short:"m" long:"max-size" description:"maximal size of the cache in bytes" default:"50M"`
		NoInMemoryRoot bool          `long:"no-in-memory-root" description:"by default all cacheable root file (non-asset files) are always cached in memory, this option disable it and put it in the LRU cache"`
	} `group:"cache-control" namespace:"cache"`

	CSPNonce struct {
		Disable       bool     `long:"nonce-disable" description:"Disable CSP Nonce generation"`
		NoncedFiles   []string `short:"O" long:"nonced" description:"list of nonced file" default:"index.html"`
		DefaultPolicy string   `long:"policy" description:"CSP to use" default:"default-src 'self'; style-src 'self' 'nonce-CSP_NONCE'; script-src 'self' 'nonce-CSP_NONCE'"`
	} `group:"csp-nonce" namespace:"csp"`
}
