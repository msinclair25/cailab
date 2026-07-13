// Package catalog exposes the immutable scenario catalog compiled into cailab.
package catalog

import (
	"embed"
	"io/fs"
)

//go:embed */scenario.yaml
var files embed.FS

// Files returns the read-only built-in scenario catalog.
func Files() fs.FS {
	return files
}
