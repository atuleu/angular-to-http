package ath

import (
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"mime"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"golang.org/x/exp/slices"
)

type routeBuilder struct {
	root               string
	config             Config
	t                  *template.Template
	enabledCompression []Compression
	permanent, sized   Cache
}

func BuildRoutes(config Config) (map[string]Route, error) {
	tmpl, err := template.New("CSP").Parse(config.CSP.Policy)
	if err != nil {
		return nil, err
	}

	sized := NewCache(int64(config.Cache.MaxSize))
	var permanent Cache
	if config.Cache.NoInMemoryRoot == true {
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
	err := filepath.WalkDir(b.root,
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

	if b.config.CSP.Disable != false &&
		slices.Contains(b.config.CSP.NoncedPath, target) == true {

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
			enabledCompression: b.getCompression(fileinfo),
		},
		filepath: path,
		modtime:  fileinfo.ModTime(),
		cache:    b.getCache(path),
		maxAge:   b.getMaxAge(path),
	}, nil
}

func (b *routeBuilder) inRoot(path string) bool {
	return filepath.Dir(path) == filepath.Clean(b.root)
}

func (b *routeBuilder) getCache(path string) Cache {
	if b.inRoot(path) == true {
		return b.permanent
	}
	return b.sized
}

var hashRx = regexp.MustCompile(`\A\.[[:xdigit:]]+\z`)

func isVersionned(path string) bool {
	ext := filepath.Ext(path)
	midExt := filepath.Ext(strings.TrimSuffix(path, ext))
	return hashRx.MatchString(midExt)
}

func (b *routeBuilder) getMaxAge(path string) int {
	if filepath.Ext(path) == ".html" && b.inRoot(path) && isVersionned(path) == false {
		return 0
	}
	return int(b.config.Cache.MaxAge.Seconds())
}

var allowedCompression = map[string]bool{
	".txt":         true,
	".js":          true,
	".map":         true,
	".html":        true,
	".json":        true,
	".webmanifest": true,
	".svg":         true,
	".ttf":         true,
	".otf":         true,
	".xml":         true,
}

func (b *routeBuilder) getCompression(fileinfo fs.FileInfo) []Compression {
	if allowedCompression[filepath.Ext(fileinfo.Name())] == true &&
		fileinfo.Size() >= int64(b.config.Compression.Threshold) {
		return b.enabledCompression
	}
	return nil
}

func buildTarget(root, path string) string {
	target, _ := filepath.Rel(root, path)
	return "/" + target
}
