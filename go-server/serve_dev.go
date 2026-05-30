//go:build !release

package main

import (
	"net/http"
	"os"
	"path/filepath"
)

func setupStaticHandlers() {
	// Serve the raw UI folder directly (No Svelte/Build needed)
	uiFs := http.FileServer(http.Dir("../ui"))
	http.Handle("/ui/", http.StripPrefix("/ui/", uiFs))

	// Serve SvelteKit build folder with SPA fallback
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join("../build", filepath.Clean(r.URL.Path))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// If file doesn't exist, serve SPA fallback (index.html)
			http.ServeFile(w, r, "../build/index.html")
			return
		}
		// Otherwise serve the static file
		http.FileServer(http.Dir("../build")).ServeHTTP(w, r)
	})
}
