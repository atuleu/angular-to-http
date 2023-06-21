package ath

import (
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"io/ioutil"
	"mime"
	"path/filepath"
	"strings"

	"golang.org/x/exp/slices"
)

type routeBuilder struct {
	root               string
	config             Config
	t                  *template.Template
	enabledCompression []Compression
	permanent, sized   Cache
}

func BuildRoute(config Config) (map[string]Route, error) {
	tmpl, err := template.New("CSP").Parse(config.CSPNonce.DefaultPolicy)
	if err != nil {
		return nil, err
	}

	sized := NewCache(int64(config.CacheControl.MaxSize))
	var permanent Cache
	if config.CacheControl.NoInMemoryRoot == true {
		permanent = sized
	} else {
		permanent = NewCache(-1)
	}

	return (&routeBuilder{
		root:               config.Args.Directory,
		config:             config,
		t:                  tmpl,
		enabledCompression: config.EnabledCompressions(),
		permanent:          permanent,
		sized:              sized,
	}).buildRoutes()

}

func (b *routeBuilder) buildRoutes() (map[string]Route, error) {
	res := make(map[string]Route)
	err := fs.WalkDir(b.root,
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() == true {
				return nil
			}

			target, route, err := b.buildRoute(path, d)
			if err != nil {
				return err
			}
			res[target] = route
			return nil
		})
	return res, err
}

var ErrNonNonceable = errors.New("route is not nonceable")

func (b *routeBuilder) buildRoute(path string, d fs.DirEntry) (string, Route, error) {
	target := buildTarget(b.root, path)

	if b.config.CSPNonce.Disable != false &&
		slices.Contains(b.config.CSPNonce.NoncedFiles, target) == true {

		nonced, err := b.buildNoncedRoute(path)
		if err == nil {
			return target, nonced, nil
		}

		if err != ErrNonNonceable {
			return target, nil, err
		}
	}

	route, err := b.buildStaticRoute(path, d)
	if err != nil {
		return target, nil, err
	}

	return target, route, nil
}

func (b *routeBuilder) buildNoncedRoute(path string) (Route, error) {
	content_, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open '%s': %w", path, err)
	}
	content := string(content_)

	if strings.Contains(content, "CSP_NONCE") == false {
		return nil, ErrNonNonceable
	}

	templ, err := b.t.Clone()
	if err != nil {
		return nil, err
	}

	templ, err = template.New("content").Parse(strings.ReplaceAll(content,
		"CSP_NONCE", "{{.Nonce}}"))
	if err != nil {
		return nil, err
	}

	name := filepath.Base(path)
	mime := mime.TypeByExtension(filepath.Ext(name))

	return &NoncedRoute{
		route: route{
			name:               name,
			mime:               mime,
			enabledCompression: b.enabledCompression,
		},
		template: templ,
	}, nil
}

func (b *routeBuilder) buildStaticRoute(path string, d fs.DirEntry) (Route, error) {
	name := filepath.Base(path)
	mime := mime.TypeByExtension(filepath.Ext(name))

	fileinfo, err := d.Info()
	if err != nil {
		return nil, err
	}

	return StaticRoute{
		route: route{
			name:               name,
			mime:               mime,
			enabledCompression: b.getCompression(mime),
		},
		filepath: path,
		modtime:  fileinfo.ModTime(),
		cache:    b.getCache(path),
		maxAge:   b.getMaxAge(path, mime),
	}
}

func (b *routeBuilder) inRoot(path string) bool {
	return filepath.Dir(path) == b.root
}

func (b *routeBuilder) getCache(path string) Cache {
	if b.inRoot(path) == true {
		return b.permanent
	}
	return b.sized
}

func isVersionned(path string) bool {
	ext := filepath.Ext(path)
	midExt := filepath.Ext(ext)
	return ext != midExt
}

func (b *routeBuilder) getMaxAge(path string, mime string) int {
	if mime == "text/html" && b.inRoot(path) && isVersionned(path) == false {
		return 0
	}
	return int(b.config.CacheControl.MaxAge.Seconds())
}

var allowedCompression = map[string]bool{
	"text/plain":       true,
	"text/javascript":  true,
	"text/html":        true,
	"application/json": true,
	"image/svg+xml":    true,
}

func (b *routeBuilder) getCompression(mime string, fileinfo fs.FileInfo) []Compression {
	if allowedCompression[mime] == true &&
		fileinfo.Size() >= int64(b.config.Compression.CompressionThreshold) {
		return b.enabledCompression
	}
	return nil
}

func buildTarget(root, path string) string {
	target, _ := filepath.Rel(root, path)
	return "/" + target
}
