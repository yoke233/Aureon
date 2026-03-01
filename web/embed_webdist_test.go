//go:build webdist

package webassets

import (
	"io/fs"
	"testing"
)

func TestEmbeddedFrontendMode_Webdist(t *testing.T) {
	if got := EmbeddedFrontendMode(); got != "webdist" {
		t.Fatalf("webdist embedded frontend mode mismatch, got %q want %q", got, "webdist")
	}
}

func TestDistFS_WebdistBuildProvidesIndex(t *testing.T) {
	dist, err := DistFS()
	if err != nil {
		t.Fatalf("DistFS() error = %v, want nil", err)
	}

	if _, err := fs.Stat(dist, "index.html"); err != nil {
		t.Fatalf("index.html should exist in webdist embed, got err: %v", err)
	}
}
