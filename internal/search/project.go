package search

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/zeebo/xxh3"
)

// DetectProjectRoot walks upward from cwd looking for a .draft/ directory
// or a CLAUDE.md file. Returns the first directory containing either marker,
// or cwd itself if no marker is found.
func DetectProjectRoot(cwd string) (string, error) {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolve symlinks: %w", err)
	}

	dir := resolved
	for {
		if hasMarker(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding a marker.
			return resolved, nil
		}
		dir = parent
	}
}

func hasMarker(dir string) bool {
	for _, marker := range []string{".draft", "CLAUDE.md"} {
		info, err := os.Stat(filepath.Join(dir, marker))
		if err != nil {
			continue
		}
		if marker == ".draft" && info.IsDir() {
			return true
		}
		if marker == "CLAUDE.md" && !info.IsDir() {
			return true
		}
	}
	return false
}

// IndexPath returns the path to the SQLite database for the given project root.
// If overridePath is non-empty, it is returned directly (supports --db flag).
func IndexPath(projectRoot, overridePath string) (string, error) {
	if overridePath != "" {
		return overridePath, nil
	}

	resolved, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		return "", fmt.Errorf("resolve symlinks: %w", err)
	}

	abs, err := filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	cacheDir, err := cacheBaseDir()
	if err != nil {
		return "", err
	}

	hash := xxh3.HashString128(abs)
	b := hash.Bytes()
	name := hex.EncodeToString(b[:])
	return filepath.Join(cacheDir, name+".db"), nil
}

// cacheBaseDir returns the platform-appropriate cache directory for draft indexes.
func cacheBaseDir() (string, error) {
	var base string
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("home dir: %w", err)
		}
		base = filepath.Join(home, "Library", "Caches")
	default: // linux and others
		if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
			base = xdg
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("home dir: %w", err)
			}
			base = filepath.Join(home, ".cache")
		}
	}
	return filepath.Join(base, "draft"), nil
}
