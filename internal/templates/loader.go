package templates

import (
	"io/fs"
)

// Loader defines the interface for loading template files
type Loader interface {
	// Load returns a filesystem containing the .claude directory structure
	Load() (fs.FS, error)
	// Source returns a human-readable description of where templates came from
	Source() string
}

// LoadResult contains the loaded filesystem and metadata about the source
type LoadResult struct {
	FS     fs.FS
	Source string
}
