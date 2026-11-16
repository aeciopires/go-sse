// Package web embeds the frontend assets (HTML templates, CSS and JavaScript)
// into the binary so the application can run from any working directory.
package web

import (
	"embed"
	"io/fs"
)

//go:embed templates static
var content embed.FS

// Templates returns the filesystem holding the HTML templates.
func Templates() fs.FS {
	return mustSub("templates")
}

// Static returns the filesystem holding the static assets (CSS, JS).
func Static() fs.FS {
	return mustSub("static")
}

func mustSub(dir string) fs.FS {
	sub, err := fs.Sub(content, dir)
	if err != nil {
		// Only possible if the embed directive above is changed incorrectly.
		panic(err)
	}
	return sub
}
