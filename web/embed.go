package webassets

import (
	"embed"
	"io/fs"
)

// DistFS contains the SPA build output from web/dist.
//
//go:embed all:dist
var distFS embed.FS

// DistFS returns a filesystem rooted at web/dist.
func DistFS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
