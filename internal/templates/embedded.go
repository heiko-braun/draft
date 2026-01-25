package templates

import (
	"embed"
	"io/fs"
)

// EmbeddedLoader loads templates from an embedded filesystem
type EmbeddedLoader struct {
	fs     embed.FS
	prefix string
}

// NewEmbeddedLoader creates an EmbeddedLoader with the given embedded filesystem.
// The prefix is the path within the embed.FS where templates are located (e.g., "templates")
func NewEmbeddedLoader(fs embed.FS, prefix string) *EmbeddedLoader {
	return &EmbeddedLoader{
		fs:     fs,
		prefix: prefix,
	}
}

// Load returns the embedded filesystem
func (e *EmbeddedLoader) Load() (fs.FS, error) {
	// Return a sub-filesystem rooted at the prefix
	if e.prefix != "" {
		return fs.Sub(e.fs, e.prefix)
	}
	return e.fs, nil
}

// Source returns a description of the template source
func (e *EmbeddedLoader) Source() string {
	return "embedded templates (fallback)"
}
