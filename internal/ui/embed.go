package ui

import (
	"embed"
	"io/fs"
)

//go:embed web/*
var webFS embed.FS

// webRoot es el filesystem de los assets servidos en "/".
func webRoot() fs.FS {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		// Impossible si el //go:embed resolvió: fallback al raw FS.
		return webFS
	}
	return sub
}
