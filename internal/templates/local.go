package templates

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// LocalLoader loads templates from a local filesystem path
type LocalLoader struct {
	path string
}

// NewLocalLoader creates a LocalLoader for the given path.
// The path should contain a .claude/ subdirectory.
func NewLocalLoader(path string) *LocalLoader {
	return &LocalLoader{path: path}
}

// Load validates the directory structure and returns the filesystem
func (l *LocalLoader) Load() (fs.FS, error) {
	// Validate that the path exists
	info, err := os.Stat(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("DRAFT_TEMPLATES directory does not exist: %s", l.path)
		}
		return nil, fmt.Errorf("failed to access DRAFT_TEMPLATES directory: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("DRAFT_TEMPLATES is not a directory: %s", l.path)
	}

	// Validate required subdirectories exist
	claudeDir := filepath.Join(l.path, ".claude")
	if err := validateDirectory(claudeDir); err != nil {
		return nil, fmt.Errorf("DRAFT_TEMPLATES missing .claude/ subdirectory: %s", l.path)
	}

	commandsDir := filepath.Join(claudeDir, "commands")
	if err := validateDirectory(commandsDir); err != nil {
		return nil, fmt.Errorf("DRAFT_TEMPLATES missing .claude/commands/ subdirectory: %s", l.path)
	}

	specsDir := filepath.Join(claudeDir, "specs")
	if err := validateDirectory(specsDir); err != nil {
		return nil, fmt.Errorf("DRAFT_TEMPLATES missing .claude/specs/ subdirectory: %s", l.path)
	}

	// Return the OS filesystem rooted at the provided path
	return os.DirFS(l.path), nil
}

// Source returns a description of the template source
func (l *LocalLoader) Source() string {
	return fmt.Sprintf("local directory: %s", l.path)
}

// validateDirectory checks if a directory exists
func validateDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", path)
	}
	return nil
}
