package server

import (
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/alexandrmotologa/errpulse/web"
)

// SPAHandler serves embedded static assets from web.DistFS with SPA fallback to index.html.
type SPAHandler struct {
	fileSystem http.FileSystem
	rawFS      fs.FS
}

// NewSPAHandler creates a new static asset handler.
func NewSPAHandler() *SPAHandler {
	distSub, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		// Fallback to empty FS if sub failed
		distSub = web.DistFS
	}

	return &SPAHandler{
		fileSystem: http.FS(distSub),
		rawFS:      distSub,
	}
}

// ServeHTTP serves static files or returns index.html for client-side routing.
func (h *SPAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cleanPath := path.Clean(r.URL.Path)
	if cleanPath == "/" {
		cleanPath = "index.html"
	} else {
		cleanPath = strings.TrimPrefix(cleanPath, "/")
	}

	// Check if file exists in embedded filesystem
	file, err := h.fileSystem.Open(cleanPath)
	if err == nil {
		defer file.Close()
		stat, statErr := file.Stat()
		if statErr == nil && !stat.IsDir() {
			http.FileServer(h.fileSystem).ServeHTTP(w, r)
			return
		}
	}

	// Fallback to index.html for SPA routes
	indexFile, err := h.fileSystem.Open("index.html")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, "ErrPulse UI assets not embedded", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer indexFile.Close()

	var modTime time.Time
	if s, err := indexFile.Stat(); err == nil {
		modTime = s.ModTime()
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, "index.html", modTime, indexFile)
}
