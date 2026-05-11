package review

import (
	"crypto/sha256"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// RepoInfo holds identity information about a git repository.
type RepoInfo struct {
	// Root is the absolute path to the repository's working tree root.
	Root string

	// RemoteURL is the raw remote origin URL as configured in git.
	RemoteURL string

	// NormalizedURL is the remote URL after normalization (lowercased host,
	// stripped of credentials, trailing .git, etc.).
	NormalizedURL string

	// RepoID is the first 12 hex characters of the SHA-256 hash of
	// NormalizedURL, used to derive filesystem paths for worktrees.
	RepoID string
}

// DetectRepo detects the git repository containing dir and returns identity
// information including the normalized remote URL and derived repo-id.
func DetectRepo(dir string) (*RepoInfo, error) {
	root, err := gitRepoRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("detecting git repo root: %w", err)
	}

	remoteURL, err := gitRemoteOriginURL(root)
	if err != nil {
		return nil, fmt.Errorf("reading remote origin URL: %w", err)
	}

	normalized := NormalizeRemoteURL(remoteURL)
	repoID := deriveRepoID(normalized)

	return &RepoInfo{
		Root:          root,
		RemoteURL:     remoteURL,
		NormalizedURL: normalized,
		RepoID:        repoID,
	}, nil
}

// gitRepoRoot returns the top-level directory of the git working tree
// containing dir.
func gitRepoRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository (or no git installed): %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// gitRemoteOriginURL returns the URL of the "origin" remote for the repo at
// the given root directory.
func gitRemoteOriginURL(root string) (string, error) {
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("no 'origin' remote configured: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// sshURLPattern matches SSH-style git URLs like git@github.com:user/repo.git
var sshURLPattern = regexp.MustCompile(`^[\w.-]+@([\w.-]+):(.*?)(?:\.git)?$`)

// httpsURLPattern matches HTTPS-style git URLs
var httpsURLPattern = regexp.MustCompile(`^https?://(?:[^@]+@)?([\w.-]+)/(.+?)(?:\.git)?/?$`)

// NormalizeRemoteURL normalizes a git remote URL to a canonical form for
// consistent hashing. The normalization:
//   - Converts SSH URLs (git@host:path) to a canonical form
//   - Strips credentials from HTTPS URLs
//   - Removes trailing .git suffix
//   - Lowercases the host
//   - Produces a consistent "host/path" output
func NormalizeRemoteURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)

	// Try SSH pattern: git@github.com:user/repo.git
	if m := sshURLPattern.FindStringSubmatch(rawURL); m != nil {
		host := strings.ToLower(m[1])
		path := strings.TrimSuffix(m[2], "/")
		return host + "/" + path
	}

	// Try HTTPS pattern: https://github.com/user/repo.git
	if m := httpsURLPattern.FindStringSubmatch(rawURL); m != nil {
		host := strings.ToLower(m[1])
		path := strings.TrimSuffix(m[2], "/")
		return host + "/" + path
	}

	// Fallback: return as-is (unlikely in practice).
	return rawURL
}

// deriveRepoID computes the repo-id from a normalized URL. It is the first 12
// hex characters of the SHA-256 hash of the normalized URL string.
func deriveRepoID(normalizedURL string) string {
	h := sha256.Sum256([]byte(normalizedURL))
	return fmt.Sprintf("%x", h[:6]) // 6 bytes = 12 hex chars
}

// OwnerRepo extracts the GitHub owner and repo name from the normalized URL.
// For a normalized URL like "github.com/owner/repo", returns ("owner", "repo").
func (r *RepoInfo) OwnerRepo() (string, string) {
	parts := strings.Split(r.NormalizedURL, "/")
	if len(parts) >= 3 {
		return parts[1], parts[2]
	}
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", r.NormalizedURL
}
