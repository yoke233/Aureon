//go:build webdist

package webassets

import (
	"embed"
	"io/fs"
)

// DistFS contains the SPA build output from web/dist.
//
//go:embed all:dist
var distFS embed.FS

// EmbeddedFrontendMode identifies which embedded frontend source is compiled.
func EmbeddedFrontendMode() string {
	return "webdist"
}

// DistFS returns a filesystem rooted at web/dist.
func DistFS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
