package ath

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Route interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
	PreCache()
}

type FileRoute struct {
	path string

	config *Config
	cache  Cache

	modtime time.Time
	mime    string

	isCacheable    bool
	isCompressible bool
}

func (r FileRoute) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if r.isCacheable == true && r.config.CacheControl.MaxAge >= 1*time.Second {
		w.Header().Add("Cache-Control",
			fmt.Sprintf("max-age=%d", int(r.config.CacheControl.MaxAge.Seconds())))
	} else {
		w.Header().Add("Cache-Control", "no-store")
	}

	comp := r.findCompression(w, req)
	comp.WriteEncoding(w)

	data, err := r.cache.Get(comp.AddExtension(r.path), r.readFile(comp))
	if err != nil {
		log.Printf("could not read %s: %s", r.path, err)
		http.Error(w, "read error", http.StatusInternalServerError)
		return
	}

	http.ServeContent(w, req, r.path, r.modtime, bytes.NewReader(data.([]byte)))
}

func (r FileRoute) PreCache() {
	compressions := []Compression{Identity}

	if r.isCompressible == true {
		compressions = append(compressions, r.config.EnabledCompressions()...)
	}

	for _, comp := range compressions {
		r.cache.Get(comp.AddExtension(r.path), r.readFile(comp))
	}
}

func (r FileRoute) findCompression(w http.ResponseWriter, req *http.Request) Compression {
	if r.isCacheable == false {
		return Identity
	}
	// TODO set compressions accordingly

	return Identity
}

func (r FileRoute) readFile(compression Compression) func() (any, error) {
	return func() (any, error) {
		file, err := os.Open(filepath.Join(r.config.Args.Directory, r.path))
		if err != nil {
			return nil, err
		}
		defer file.Close()
		return compression.Compress(file)
	}
}
