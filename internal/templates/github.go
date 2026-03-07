package templates

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

const (
	githubAPITimeout = 10 * time.Second
	defaultOwner     = "heiko-braun"
	defaultRepo      = "draft"
)

// GitHubLoader loads templates from a GitHub release
type GitHubLoader struct {
	owner   string
	repo    string
	version string // empty string means "latest"
}

// NewGitHubLoader creates a GitHubLoader for the specified repository and version.
// If version is empty, it will fetch from the latest release.
func NewGitHubLoader(owner, repo, version string) *GitHubLoader {
	if owner == "" {
		owner = defaultOwner
	}
	if repo == "" {
		repo = defaultRepo
	}
	return &GitHubLoader{
		owner:   owner,
		repo:    repo,
		version: version,
	}
}

// Load fetches the release tarball from GitHub and extracts the .claude directory
func (g *GitHubLoader) Load() (fs.FS, error) {
	ctx, cancel := context.WithTimeout(context.Background(), githubAPITimeout)
	defer cancel()

	// Get the tarball URL from the releases API
	tarballURL, err := g.getTarballURL(ctx)
	if err != nil {
		return nil, err
	}

	// Download and extract the tarball
	templateFS, err := g.downloadAndExtract(ctx, tarballURL)
	if err != nil {
		return nil, err
	}

	return templateFS, nil
}

// Source returns a description of the template source
func (g *GitHubLoader) Source() string {
	if g.version == "" {
		return fmt.Sprintf("GitHub release: %s/%s@latest", g.owner, g.repo)
	}
	return fmt.Sprintf("GitHub release: %s/%s@%s", g.owner, g.repo, g.version)
}

// getTarballURL fetches the tarball URL from GitHub's releases API
func (g *GitHubLoader) getTarballURL(ctx context.Context) (string, error) {
	var apiURL string
	if g.version == "" {
		apiURL = fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", g.owner, g.repo)
	} else {
		apiURL = fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", g.owner, g.repo, g.version)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch release info from GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		if g.version == "" {
			return "", fmt.Errorf("no releases found for %s/%s", g.owner, g.repo)
		}
		return "", fmt.Errorf("release %s not found for %s/%s", g.version, g.owner, g.repo)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release struct {
		TarballURL string `json:"tarball_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse GitHub API response: %w", err)
	}

	if release.TarballURL == "" {
		return "", fmt.Errorf("no tarball URL in release")
	}

	return release.TarballURL, nil
}

// downloadAndExtract downloads the tarball and extracts the .claude directory into an in-memory FS
func (g *GitHubLoader) downloadAndExtract(ctx context.Context, tarballURL string) (fs.FS, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", tarballURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download tarball: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download tarball: status %d", resp.StatusCode)
	}

	// Decompress gzip
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress tarball: %w", err)
	}
	defer gzr.Close()

	// Extract .claude directory from tar
	memFS := newMemFS()
	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tarball: %w", err)
		}

		// GitHub tarballs have a root directory like "owner-repo-commitsha/"
		// We need to strip that and only keep files under .claude/
		parts := strings.SplitN(header.Name, "/", 2)
		if len(parts) < 2 {
			continue
		}

		relPath := parts[1] // Everything after the root directory

		// Only extract template files (not project-specific specs)
		// Support both old (.claude/specs/) and new (specs/) locations for backward compatibility
		allowedFiles := []string{
			".claude/commands/spec.md",
			".claude/commands/implement.md",
			".claude/commands/refine.md",
			".claude/commands/verify.md",
			".claude/agents/verify-agent.md",
			".cursor/skills/spec/SKILL.md",
			".cursor/skills/implement/SKILL.md",
			".cursor/skills/refine/SKILL.md",
			".cursor/skills/verify/SKILL.md",
			".cursor/specs/TEMPLATE.md",
			".principles/design-principles.md",
			".principles/review-role.md",
			"specs/TEMPLATE.md",         // New location
			".claude/specs/TEMPLATE.md", // Old location (for backward compatibility)
		}

		allowed := false
		var targetPath string
		for _, allowedFile := range allowedFiles {
			if relPath == allowedFile {
				allowed = true
				// Normalize old location to new location
				if relPath == ".claude/specs/TEMPLATE.md" {
					targetPath = "specs/TEMPLATE.md"
				} else {
					targetPath = relPath
				}
				break
			}
		}

		if !allowed {
			continue
		}

		if header.Typeflag == tar.TypeDir {
			continue // We'll create directories as needed
		}

		if header.Typeflag == tar.TypeReg {
			content, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("failed to read file %s: %w", header.Name, err)
			}

			// Use normalized target path (handles old -> new location mapping)
			memFS.addFile(targetPath, content)
		}
	}

	if len(memFS.files) == 0 {
		return nil, fmt.Errorf("no template files found in release tarball")
	}

	return memFS, nil
}

// memFS is a simple in-memory filesystem implementation
type memFS struct {
	files map[string][]byte
}

func newMemFS() *memFS {
	return &memFS{
		files: make(map[string][]byte),
	}
}

func (m *memFS) addFile(path string, content []byte) {
	m.files[path] = content
}

func (m *memFS) Open(name string) (fs.File, error) {
	// Clean the path
	name = filepath.Clean(name)
	if name == "." {
		return &memDir{fs: m, path: "."}, nil
	}

	// Check if it's a file
	if content, ok := m.files[name]; ok {
		return &memFile{
			name:    filepath.Base(name),
			content: content,
		}, nil
	}

	// Check if it's a directory by seeing if any files have this prefix
	prefix := name + "/"
	for path := range m.files {
		if strings.HasPrefix(path, prefix) || path == name {
			return &memDir{fs: m, path: name}, nil
		}
	}

	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

// memFile represents an in-memory file
type memFile struct {
	name    string
	content []byte
	offset  int64
}

func (f *memFile) Stat() (fs.FileInfo, error) {
	return &memFileInfo{name: f.name, size: int64(len(f.content))}, nil
}

func (f *memFile) Read(b []byte) (int, error) {
	if f.offset >= int64(len(f.content)) {
		return 0, io.EOF
	}
	n := copy(b, f.content[f.offset:])
	f.offset += int64(n)
	return n, nil
}

func (f *memFile) Close() error {
	return nil
}

// memDir represents an in-memory directory
type memDir struct {
	fs   *memFS
	path string
}

func (d *memDir) Stat() (fs.FileInfo, error) {
	return &memFileInfo{name: filepath.Base(d.path), size: 0, isDir: true}, nil
}

func (d *memDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.path, Err: fs.ErrInvalid}
}

func (d *memDir) Close() error {
	return nil
}

func (d *memDir) ReadDir(n int) ([]fs.DirEntry, error) {
	var entries []fs.DirEntry
	seen := make(map[string]bool)

	prefix := d.path
	if prefix != "." {
		prefix = prefix + "/"
	} else {
		prefix = ""
	}

	for path := range d.fs.files {
		if !strings.HasPrefix(path, prefix) {
			continue
		}

		rel := strings.TrimPrefix(path, prefix)
		parts := strings.SplitN(rel, "/", 2)
		name := parts[0]

		if name == "" || seen[name] {
			continue
		}
		seen[name] = true

		isDir := len(parts) > 1
		fullPath := filepath.Join(prefix, name)

		if isDir {
			entries = append(entries, &memDirEntry{
				name:  name,
				isDir: true,
			})
		} else {
			entries = append(entries, &memDirEntry{
				name:  name,
				isDir: false,
				size:  int64(len(d.fs.files[fullPath])),
			})
		}
	}

	return entries, nil
}

// memFileInfo implements fs.FileInfo
type memFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (fi *memFileInfo) Name() string       { return fi.name }
func (fi *memFileInfo) Size() int64        { return fi.size }
func (fi *memFileInfo) Mode() fs.FileMode  { return 0644 }
func (fi *memFileInfo) ModTime() time.Time { return time.Time{} }
func (fi *memFileInfo) IsDir() bool        { return fi.isDir }
func (fi *memFileInfo) Sys() interface{}   { return nil }

// memDirEntry implements fs.DirEntry
type memDirEntry struct {
	name  string
	isDir bool
	size  int64
}

func (de *memDirEntry) Name() string      { return de.name }
func (de *memDirEntry) IsDir() bool       { return de.isDir }
func (de *memDirEntry) Type() fs.FileMode { return 0 }
func (de *memDirEntry) Info() (fs.FileInfo, error) {
	return &memFileInfo{name: de.name, size: de.size, isDir: de.isDir}, nil
}
