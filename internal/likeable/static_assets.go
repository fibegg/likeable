package likeable

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var embeddedWeb embed.FS

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if s.config.WebDir != "" {
		file := path.Clean(r.URL.Path)
		if file == "." || file == "/" {
			file = "index.html"
		}
		full := strings.TrimPrefix(file, "/")
		if _, err := os.Stat(filepath.Join(s.config.WebDir, full)); err == nil {
			http.ServeFile(w, r, filepath.Join(s.config.WebDir, full))
			return
		}
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
		http.FileServer(http.FS(sub)).ServeHTTP(w, r)
		return
	}
	data, err := embeddedWeb.ReadFile("web-dist/index.html")
	if err != nil {
		writeError(w, http.StatusNotFound, "frontend is not built")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}
