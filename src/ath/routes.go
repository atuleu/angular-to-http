package ath

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type Route interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
	PreCache()
}

type route struct {
	name               string
	mime               string
	enabledCompression []Compression
}

type StaticRoute struct {
	route

	filepath string

	modtime time.Time

	cache Cache

	maxAge int
}

func (r StaticRoute) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	comp := r.findCompression(w, req)
	data, err := r.cache.Get(comp.AddExtension(r.filepath), r.readFile(comp))
	if err != nil {
		log.Printf("could not read %s: %s", r.filepath, err)
		http.Error(w, "read error", http.StatusInternalServerError)
		return
	}

	if r.maxAge > 0 {
		w.Header().Add("Cache-Control", fmt.Sprintf("max-age=%d", r.maxAge))
	} else {
		w.Header().Add("Cache-Control", "no-store")
	}
	comp.WriteEncodingHeader(w)

	http.ServeContent(w, req, r.name, r.modtime, bytes.NewReader(data))
}

func (r StaticRoute) PreCache() {
	compressions := append(r.enabledCompression, Identity)

	for _, comp := range compressions {
		r.cache.Get(comp.AddExtension(r.filepath), r.readFile(comp))
	}
}

func (r route) findCompression(w http.ResponseWriter, req *http.Request) Compression {
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
			return nil, fmt.Errorf("open %s: %w", r.filepath, err)
		}
		defer file.Close()
		res, err := compression.Compress(file)
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
	if err != nil {
		log.Printf("could not generate nonce: %s", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	comp := r.findCompression(w, req)

	response := bytes.NewBuffer(nil)
	csp := bytes.NewBuffer(nil)

	err = r.template.ExecuteTemplate(comp.Wrap(response), "content", nonce)
	if err != nil {
		log.Printf("could not execute response template for  %s: %s", r.name, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	err = r.template.ExecuteTemplate(csp, "CSP", nonce)
	if err != nil {
		log.Printf("could not execute CSP template for %s: %s", r.name, err)
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

func (r NoncedRoute) PreCache() {}

func (r NoncedRoute) generateNonce() (Nonce, error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return Nonce{}, err
	}
	return Nonce{base64.RawURLEncoding.EncodeToString(nonce)}, nil
}
