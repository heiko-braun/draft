package review

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// WorktreeResult holds the paths to the two worktrees managed by draft review.
type WorktreeResult struct {
	// DocsPath is the absolute path to the sparse document worktree.
	DocsPath string

	// ReviewsPath is the absolute path to the review data worktree.
	ReviewsPath string

	// RepoID is the repository identifier used in the worktree path.
	RepoID string
}

// draftBaseDir returns the base directory for draft data (~/.draft).
func draftBaseDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home directory: %w", err)
	}
	return filepath.Join(home, ".draft"), nil
}

// worktreeBaseDir returns the worktree base directory for a given repo-id.
func worktreeBaseDir(repoID string) (string, error) {
	base, err := draftBaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "worktrees", repoID), nil
}

// EnsureWorktrees creates or verifies both the document and review worktrees
// for the repository at repoRoot. sourceBranch is the branch from which
// documents are read (e.g. "main" or "origin/main").
//
// If worktrees already exist and are valid, they are reused. If they are
// broken (e.g. the parent clone was moved), they are removed and re-created.
func EnsureWorktrees(repoRoot string, cfg ReviewConfig, sourceBranch string) (*WorktreeResult, error) {
	info, err := DetectRepo(repoRoot)
	if err != nil {
		return nil, err
	}

	base, err := worktreeBaseDir(info.RepoID)
	if err != nil {
		return nil, err
	}

	docsPath := filepath.Join(base, "docs")
	reviewsPath := filepath.Join(base, "reviews")

	// Ensure base directory exists.
	if err := os.MkdirAll(base, 0755); err != nil {
		return nil, fmt.Errorf("creating worktree base directory: %w", err)
	}

	// --- Document worktree ---
	if err := ensureDocsWorktree(repoRoot, docsPath, cfg.DocumentPaths, sourceBranch); err != nil {
		return nil, fmt.Errorf("ensuring document worktree: %w", err)
	}

	// --- Review worktree ---
	if err := ensureReviewsWorktree(repoRoot, reviewsPath); err != nil {
		return nil, fmt.Errorf("ensuring reviews worktree: %w", err)
	}

	return &WorktreeResult{
		DocsPath:    docsPath,
		ReviewsPath: reviewsPath,
		RepoID:      info.RepoID,
	}, nil
}

// ensureDocsWorktree creates or verifies the sparse document worktree.
func ensureDocsWorktree(repoRoot, docsPath string, docPaths []string, sourceBranch string) error {
	valid := verifyWorktree(repoRoot, docsPath)
	if valid {
		return nil
	}

	// Remove broken worktree if directory exists.
	if err := removeWorktreeIfExists(repoRoot, docsPath); err != nil {
		return err
	}

	// Create worktree with --no-checkout and --detach for sparse checkout.
	// --detach avoids the "branch already checked out" error since the main
	// worktree typically has the source branch checked out.
	if err := run(repoRoot, "git", "worktree", "add", "--no-checkout", "--detach", docsPath, sourceBranch); err != nil {
		return fmt.Errorf("creating document worktree: %w", err)
	}

	// Configure sparse checkout.
	if err := configureSparseCheckout(docsPath, docPaths); err != nil {
		return fmt.Errorf("configuring sparse checkout: %w", err)
	}

	// Perform the checkout now that sparse-checkout is configured.
	if err := run(docsPath, "git", "checkout"); err != nil {
		return fmt.Errorf("checking out sparse documents: %w", err)
	}

	return nil
}

// configureSparseCheckout enables sparse checkout in the worktree and writes
// the given paths to the sparse-checkout file.
func configureSparseCheckout(worktreePath string, paths []string) error {
	// Enable sparse checkout.
	if err := run(worktreePath, "git", "sparse-checkout", "init"); err != nil {
		return fmt.Errorf("initializing sparse checkout: %w", err)
	}

	// Set the sparse checkout paths.
	args := append([]string{"sparse-checkout", "set"}, paths...)
	if err := run(worktreePath, "git", args...); err != nil {
		return fmt.Errorf("setting sparse checkout paths: %w", err)
	}

	return nil
}

// ensureReviewsWorktree creates or verifies the review data worktree.
func ensureReviewsWorktree(repoRoot, reviewsPath string) error {
	valid := verifyWorktree(repoRoot, reviewsPath)
	if valid {
		return nil
	}

	// Remove broken worktree if directory exists.
	if err := removeWorktreeIfExists(repoRoot, reviewsPath); err != nil {
		return err
	}

	// Create worktree tracking the reviews branch.
	if err := run(repoRoot, "git", "worktree", "add", reviewsPath, BranchName); err != nil {
		return fmt.Errorf("creating reviews worktree: %w", err)
	}

	return nil
}

// verifyWorktree checks whether the worktree at wtPath is a valid git worktree
// associated with the repo at repoRoot.
func verifyWorktree(repoRoot, wtPath string) bool {
	// Check if directory exists.
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		return false
	}

	// Check if it's listed in git worktree list.
	worktrees, err := listWorktrees(repoRoot)
	if err != nil {
		return false
	}

	absPath, err := filepath.Abs(wtPath)
	if err != nil {
		return false
	}
	// Resolve symlinks for comparison (e.g. macOS /var -> /private/var).
	resolvedAbs, _ := filepath.EvalSymlinks(absPath)

	for _, wt := range worktrees {
		resolvedWt, _ := filepath.EvalSymlinks(wt)
		if resolvedWt == resolvedAbs {
			// Verify the directory is actually a valid git directory.
			cmd := exec.Command("git", "rev-parse", "--git-dir")
			cmd.Dir = wtPath
			if err := cmd.Run(); err != nil {
				return false
			}
			return true
		}
	}

	return false
}

// listWorktrees returns the absolute paths of all worktrees for the repo at
// repoRoot by parsing `git worktree list --porcelain`.
func listWorktrees(repoRoot string) ([]string, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing worktrees: %w", err)
	}

	var paths []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "worktree ") {
			paths = append(paths, strings.TrimPrefix(line, "worktree "))
		}
	}

	return paths, nil
}

// removeWorktreeIfExists removes a broken or stale worktree. It first tries
// `git worktree remove`, then falls back to manual cleanup.
func removeWorktreeIfExists(repoRoot, wtPath string) error {
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		return nil
	}

	// Try git worktree remove --force (handles broken worktrees).
	cmd := exec.Command("git", "worktree", "remove", "--force", wtPath)
	cmd.Dir = repoRoot
	if err := cmd.Run(); err != nil {
		// If git worktree remove fails, clean up manually.
		if err := os.RemoveAll(wtPath); err != nil {
			return fmt.Errorf("removing broken worktree directory: %w", err)
		}
		// Prune stale worktree entries.
		_ = run(repoRoot, "git", "worktree", "prune")
	}

	return nil
}

// UpdateDocsWorktree fetches the latest from origin and checks out the
// source branch in the document worktree.
func UpdateDocsWorktree(result *WorktreeResult, sourceBranch string) error {
	// Fetch latest from origin.
	if err := run(result.DocsPath, "git", "fetch", "origin"); err != nil {
		return fmt.Errorf("fetching origin in docs worktree: %w", err)
	}

	// Checkout the latest version of the source branch.
	ref := "origin/" + sourceBranch
	if err := run(result.DocsPath, "git", "checkout", ref, "--"); err != nil {
		return fmt.Errorf("checking out %s in docs worktree: %w", ref, err)
	}

	return nil
}

// UpdateReviewsWorktree fetches the latest review data from origin and
// fast-forwards the review worktree.
func UpdateReviewsWorktree(result *WorktreeResult) error {
	// Fetch latest from origin.
	if err := run(result.ReviewsPath, "git", "fetch", "origin", BranchName); err != nil {
		return fmt.Errorf("fetching review branch: %w", err)
	}

	// Fast-forward merge (--ff-only to avoid merge commits).
	cmd := exec.Command("git", "merge", "--ff-only", "origin/"+BranchName)
	cmd.Dir = result.ReviewsPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("fast-forwarding reviews worktree: %w\n%s", err, string(out))
	}

	return nil
}

// LiveModifications detects local modifications in the user's working tree
// for the given document paths. It returns the relative paths of modified
// files that fall within the configured document directories.
func LiveModifications(repoRoot string, docPaths []string) ([]string, error) {
	// Use git status --porcelain to get modified/added files.
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running git status: %w", err)
	}

	var modified []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 4 {
			continue
		}
		// porcelain format: XY filename
		// The file path starts at position 3.
		filePath := strings.TrimSpace(line[3:])
		// Handle renamed files (old -> new).
		if idx := strings.Index(filePath, " -> "); idx >= 0 {
			filePath = filePath[idx+4:]
		}

		if isUnderDocPaths(filePath, docPaths) {
			modified = append(modified, filePath)
		}
	}

	return modified, nil
}

// isUnderDocPaths checks whether a file path falls under one of the configured
// document directories.
func isUnderDocPaths(filePath string, docPaths []string) bool {
	for _, dp := range docPaths {
		// Document paths typically end with "/", e.g. "specs/", "docs/".
		prefix := strings.TrimSuffix(dp, "/")
		if strings.HasPrefix(filePath, prefix+"/") || filePath == prefix {
			return true
		}
	}
	return false
}
