// Package frontend embeds the built Wails GUI assets so cmd/valorant-rpc-gui
// can serve them from the binary.
package frontend

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Assets returns the built frontend rooted at index.html.
func Assets() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic("frontend: dist not embedded; run the frontend build: " + err.Error())
	}
	return sub
}
