//go:build release

package main

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

//go:embed embed_build embed_ui
var embedFS embed.FS

func setupStaticHandlers() {
	// Serve embedded ui folder
	uiSubFS, err := fs.Sub(embedFS, "embed_ui")
	if err != nil {
		panic(err)
	}
	http.Handle("/ui/", http.StripPrefix("/ui/", http.FileServer(http.FS(uiSubFS))))

	// Serve embedded SvelteKit build folder
	buildSubFS, err := fs.Sub(embedFS, "embed_build")
	if err != nil {
		panic(err)
	}
	buildFS := http.FileServer(http.FS(buildSubFS))
	
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		cleanPath := filepath.Clean(r.URL.Path)
		relPath := strings.TrimPrefix(cleanPath, "/")
		if relPath == "" || relPath == "." {
			relPath = "index.html"
		}
		
		// Check if file exists in the embedded build filesystem
		_, err := buildSubFS.Open(relPath)
		if err != nil {
			// If file doesn't exist, fallback to index.html (SPA routing)
			indexFile, err := buildSubFS.Open("index.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			defer indexFile.Close()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.Copy(w, indexFile)
			return
		}
		buildFS.ServeHTTP(w, r)
	})
}
