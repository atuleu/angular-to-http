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
	template           *template.Template
	enabledCompression []Compression
	allowedCompression map[string]bool
	permanent, sized   Cache
}

func BuildRoutes(config Config) (map[string]Route, error) {
	policy := strings.ReplaceAll(config.CSP.Policy, "CSP_NONCE", "{{.Nonce}}")
	tmpl, err := template.New("CSP").Parse(policy)
	if err != nil {
		return nil, err
	}

	sized := NewCache(int64(config.ServerCache.MaxMemorySize))
	var permanent Cache
	if config.ServerCache.RootFileInLRU == true {
		permanent = sized
	} else {
		permanent = NewCache(-1)
	}

	return (&routeBuilder{
		root:               config.Args.Directory,
		config:             config,
		template:           tmpl,
		enabledCompression: config.EnabledCompressions(),
		allowedCompression: config.AllowedCompressions(),
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

	if b.config.CSP.Disable == false &&
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

var ngCspNoncedRx = regexp.MustCompile(`ng_csp_nonced(="[^"]*")?`)

func (b *routeBuilder) buildNoncedRoute(path string) (Route, error) {
	content_, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open '%s': %w", path, err)
	}
	content := string(content_)

	if strings.Contains(content, "ng_csp_nonced") == false {
		return nil, ErrNonNonceable
	}

	templ, err := b.template.Clone()
	if err != nil {
		return nil, err
	}

	templ, err = templ.New("content").Parse(ngCspNoncedRx.ReplaceAllString(content,
		"ngCspNonce=\"{{.Nonce}}\""))
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
		filepath:     path,
		modtime:      fileinfo.ModTime(),
		cache:        b.getCache(path),
		cacheControl: b.getCacheControl(path),
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

var hashOrVersionRx = regexp.MustCompile(`\A(\.[[:xdigit:]]+|\.v[0-9]+)\z`)

func isVersionned(path string) bool {
	ext := filepath.Ext(path)
	midExt := filepath.Ext(strings.TrimSuffix(path, ext))
	return hashOrVersionRx.MatchString(midExt)
}

func (b *routeBuilder) getCacheControl(path string) string {
	if isVersionned(path) == true {
		return "max-age=31536000; immutable"
	}
	if b.config.Cache.MaxAge <= 0 {
		return "no-cache"
	}
	return fmt.Sprintf("max-age=%d; must-revalidate", b.config.Cache.MaxAge)
}

func (b *routeBuilder) getCompression(fileinfo fs.FileInfo) []Compression {
	filename := fileinfo.Name()
	ext := filepath.Ext(filename)
	if ext == ".map" && strings.HasSuffix(filename, ".js.map") {
		ext = ".js.map"
	}

	if b.allowedCompression[ext] == true &&
		fileinfo.Size() >= int64(b.config.Compression.Threshold) {
		return b.enabledCompression
	}
	return nil
}

func buildTarget(root, path string) string {
	target, _ := filepath.Rel(root, path)
	return "/" + target
}
