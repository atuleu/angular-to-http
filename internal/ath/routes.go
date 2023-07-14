package ath

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"

	"go.uber.org/zap"
)

type RouteFlag int

const (
	NONCED RouteFlag = 1 << iota
	IMMUTABLE
	COMPRESSIBLE
)

func (f RouteFlag) String() string {
	str := make([]string, 0, 3)
	if (f & COMPRESSIBLE) != 0 {
		str = append(str, "COMPRESSIBLE")
	}
	if (f & IMMUTABLE) != 0 {
		str = append(str, "IMMUTABLE")
	}
	if (f & NONCED) != 0 {
		str = append(str, "NONCED")
	}
	return strings.Join(str, ", ")
}

type Route interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
	PreCache() int64
	Flags() RouteFlag
}

type route struct {
	name               string
	mime               string
	enabledCompression []Compression
}

func (r route) Flags() RouteFlag {
	if len(r.enabledCompression) > 0 {
		return COMPRESSIBLE
	}
	return 0
}

type StaticRoute struct {
	route

	filepath string

	modtime time.Time

	cache        Cache
	cacheControl string
}

func (r StaticRoute) Flags() RouteFlag {
	res := r.route.Flags()
	if strings.Contains(r.cacheControl, "immutable") {
		return res | IMMUTABLE
	}
	return res
}

func (r StaticRoute) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	comp := r.findCompression(req)
	compFilename := comp.AddExtension(r.filepath)
	data, err := r.cache.Get(compFilename, r.readFile(comp))
	if err != nil {

		zap.L().Warn("could not read route",
			zap.String("filepath", r.filepath),
			zap.String("compression", comp.AddExtension("")),
			zap.Error(err),
		)

		http.Error(w, "read error", http.StatusInternalServerError)
		return
	}

	if len(r.cacheControl) > 0 {
		w.Header().Set("Cache-Control", r.cacheControl)
	}
	comp.WriteEncodingHeader(w)

	http.ServeContent(w, req, r.name, r.modtime, bytes.NewReader(data))
}

func (r StaticRoute) PreCache() int64 {
	compressions := append(r.enabledCompression, Identity)

	var size int64
	for _, comp := range compressions {
		data, _ := r.cache.Get(comp.AddExtension(r.filepath), r.readFile(comp))
		size += int64(cap(data))
	}
	return size
}

func (r route) findCompression(req *http.Request) Compression {
	acceptEncoding := req.Header.Get("Accept-Encoding")
	if len(r.enabledCompression) > 0 && strings.Contains(acceptEncoding, "*") == true {
		return r.enabledCompression[0]
	}

	for _, c := range r.enabledCompression {
		if strings.Contains(acceptEncoding, c.(compression).name) {
			return c
		}
	}

	return Identity
}

func (r StaticRoute) readFile(compression Compression) func() ([]byte, error) {
	return func() ([]byte, error) {
		file, err := os.Open(r.filepath)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		res, err := CompressAll(compression, file)
		if err != nil {
			return nil, fmt.Errorf("compressing %s: %w", r.filepath, err)
		}
		return res, nil
	}
}

type NoncedRoute struct {
	route

	template *template.Template
}

func (r NoncedRoute) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	nonce, err := r.generateNonce()
	log := zap.L().With(zap.String("route", r.name))

	if err != nil {
		log.Warn("could not generate nonce", zap.Error(err))

		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	comp := r.findCompression(req)

	response := bytes.NewBuffer(nil)
	csp := bytes.NewBuffer(nil)

	compWriter := comp.Wrap(response)
	err = r.template.ExecuteTemplate(compWriter, "content", nonce)
	if err != nil {
		log.Warn("could not execute response template", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	err = compWriter.Close()
	if err != nil {
		log.Error("could not compress response", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	err = r.template.ExecuteTemplate(csp, "CSP", nonce)
	if err != nil {
		log.Warn("could not execute CSP template", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Add("Cache-Control", "no-store")
	comp.WriteEncodingHeader(w)
	w.Header().Add("Content-Security-Policy", string(csp.Bytes()))

	http.ServeContent(w, req, r.name, time.Now(), bytes.NewReader(response.Bytes()))
}

type Nonce struct {
	Nonce string
}

func (r NoncedRoute) Flags() RouteFlag {
	return r.route.Flags() | NONCED
}

func (r NoncedRoute) PreCache() int64 {
	return 0
}

func (r NoncedRoute) generateNonce() (Nonce, error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return Nonce{}, err
	}
	return Nonce{base64.RawURLEncoding.EncodeToString(nonce)}, nil
}
