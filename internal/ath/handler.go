package ath

import (
	"io"
	"log"
	"net/http"
	"os"
)

type Handler struct {
	routes map[string]Route
	info   *log.Logger
}

func NewHandler(routes map[string]Route, withInfo bool) *Handler {
	var info *log.Logger
	if withInfo == true {
		info = log.New(os.Stderr, "[INFO] ", 0)
	} else {
		info = log.New(io.Discard, "", 0)
	}

	if routes == nil {
		routes = make(map[string]Route)
	}

	return &Handler{routes: routes, info: info}
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *loggingResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (h *Handler) ServeHTTP(w_ http.ResponseWriter, req *http.Request) {
	w := &loggingResponseWriter{w_, 0}
	defer func() {
		h.info.Printf("%s \"%s\" from %s as \"%s\": %d",
			req.Method, req.URL,
			req.RemoteAddr, req.UserAgent(),
			w.status)
	}()

	route, ok := h.routes[req.URL.Path]
	if ok == false {
		h.info.Printf("redirecting to '/index.html'")
		route, ok = h.routes["/index.html"]
	}

	if ok == false || req.Method != "GET" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	route.ServeHTTP(w, req)
}

func init() {
	log.SetOutput(os.Stderr)
	log.SetFlags(0)
	log.SetPrefix("[WARNING] ")
}
