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
	return &Handler{routes: routes, info: info}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	h.info.Printf("%s %s from %s as %s", req.Method, req.RequestURI, req.RemoteAddr, req.UserAgent())
	defer h.info.Println("DONE")
	route, ok := h.routes[req.RequestURI]
	if ok == false {
		h.info.Printf("serving '/index.html' instead")
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
