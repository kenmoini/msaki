package web

import (
	"embed"
	"io/fs"
)

// StaticFiles contains the embedded NextJS static export
// The 'all:' prefix is required to include directories starting with underscore (_next)
// Note: The frontend/out directory must be copied to web/static before building
//
//go:embed all:static
var staticFS embed.FS

// StaticFiles returns the embedded filesystem for serving static files
func StaticFiles() (fs.FS, error) {
	return fs.Sub(staticFS, "static")
}
