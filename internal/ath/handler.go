package ath

import (
	"net/http"

	"go.uber.org/zap"
)

type Handler struct {
	routes map[string]Route
}

func NewHandler(routes map[string]Route) *Handler {
	if routes == nil {
		routes = make(map[string]Route)
	}

	return &Handler{
		routes: routes,
	}
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *loggingResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (h *Handler) log(req *http.Request) *zap.Logger {
	return zap.L().With(
		zap.String("method", req.Method),
		zap.String("URL", req.URL.String()),
		zap.String("address", req.RemoteAddr),
		zap.String("user-agent", req.UserAgent()),
	)
}

func (h *Handler) ServeHTTP(w_ http.ResponseWriter, req *http.Request) {
	w := &loggingResponseWriter{w_, 0}
	log := h.log(req)
	defer func() {
		log.Info("request", zap.Int("status", w.status))
	}()

	route, ok := h.routes[req.URL.Path]
	if ok == false {
		log.Info("redirecting to '/index.html'")
		route, ok = h.routes["/index.html"]
	}

	if ok == false || req.Method != "GET" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	route.ServeHTTP(w, req)
}
