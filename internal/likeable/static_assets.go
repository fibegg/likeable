package likeable

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed web-dist/*
var embeddedWeb embed.FS

var fingerprintedAssetPath = regexp.MustCompile(`(?:^|/)[^/]+-[A-Za-z0-9_-]{8,}\.[A-Za-z0-9]+$`)

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if s.config.WebDir != "" {
		file := path.Clean(r.URL.Path)
		if file == "." || file == "/" {
			file = "index.html"
		}
		full := strings.TrimPrefix(file, "/")
		if _, err := os.Stat(filepath.Join(s.config.WebDir, full)); err == nil {
			setStaticHeaders(w, full)
			http.ServeFile(w, r, filepath.Join(s.config.WebDir, full))
			return
		}
		setStaticHeaders(w, "index.html")
		http.ServeFile(w, r, filepath.Join(s.config.WebDir, "index.html"))
		return
	}
	file := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if file == "" || file == "." {
		file = "index.html"
	}
	sub, _ := fs.Sub(embeddedWeb, "web-dist")
	if f, err := sub.Open(file); err == nil {
		_ = f.Close()
		setStaticHeaders(w, file)
		http.FileServer(http.FS(sub)).ServeHTTP(w, r)
		return
	}
	data, err := embeddedWeb.ReadFile("web-dist/index.html")
	if err != nil {
		writeError(w, http.StatusNotFound, "frontend is not built")
		return
	}
	setStaticHeaders(w, "index.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func setStaticHeaders(w http.ResponseWriter, file string) {
	clean := strings.TrimPrefix(path.Clean("/"+file), "/")
	switch path.Ext(clean) {
	case ".webmanifest":
		w.Header().Set("Content-Type", "application/manifest+json")
	case ".js", ".mjs":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	}

	switch {
	case clean == "service-worker.js":
		w.Header().Set("Service-Worker-Allowed", "/")
		w.Header().Set("Cache-Control", "no-cache")
	case clean == "index.html":
		w.Header().Set("Cache-Control", "no-cache")
	case clean == "manifest.webmanifest" || clean == "offline.html":
		w.Header().Set("Cache-Control", "no-cache")
	case strings.HasPrefix(clean, "assets/") && fingerprintedAssetPath.MatchString(clean):
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	case isStaticFile(clean):
		w.Header().Set("Cache-Control", "public, max-age=604800")
	}
}

func isStaticFile(file string) bool {
	switch strings.ToLower(path.Ext(file)) {
	case ".css", ".js", ".mjs", ".woff", ".woff2", ".ttf", ".otf", ".eot", ".svg", ".png", ".jpg", ".jpeg", ".gif", ".ico", ".webp", ".avif":
		return true
	default:
		return false
	}
}
