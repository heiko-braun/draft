package review

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// BranchName is the well-known name of the orphan branch used to store review data.
const BranchName = "draft/reviews"

// InitReviewBranch creates the orphan branch BranchName with the initial
// directory structure and config files. If the branch already exists (locally
// or on origin), the function returns nil without making changes.
//
// gitDir is the path to the root of the git working tree (the directory that
// contains .git). All git operations are performed in a temporary directory so
// that the caller's working tree is never modified.
func InitReviewBranch(gitDir string) error {
	exists, err := branchExists(gitDir)
	if err != nil {
		return fmt.Errorf("checking for existing branch: %w", err)
	}
	if exists {
		return nil
	}

	remoteURL, err := getRemoteURL(gitDir)
	if err != nil {
		return fmt.Errorf("determining remote URL: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "draft-review-init-*")
	if err != nil {
		return fmt.Errorf("creating temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := initBranchInTempDir(tmpDir, remoteURL); err != nil {
		return err
	}

	// Fetch the newly-pushed branch into the local repo so it is available
	// immediately.
	if err := fetchBranch(gitDir); err != nil {
		return fmt.Errorf("fetching branch into local repo: %w", err)
	}

	return nil
}

// branchExists returns true if BranchName exists either as a local ref or on
// the remote named "origin".
func branchExists(gitDir string) (bool, error) {
	// Check local branch.
	cmd := exec.Command("git", "rev-parse", "--verify", "refs/heads/"+BranchName)
	cmd.Dir = gitDir
	if err := cmd.Run(); err == nil {
		return true, nil
	}

	// Check remote branch.
	cmd = exec.Command("git", "ls-remote", "--heads", "origin", "refs/heads/"+BranchName)
	cmd.Dir = gitDir
	out, err := cmd.Output()
	if err != nil {
		// ls-remote failing likely means no remote configured — that's fine,
		// we'll try to create the branch and push will fail later with a clear
		// error message.
		return false, nil
	}
	if len(strings.TrimSpace(string(out))) > 0 {
		return true, nil
	}

	return false, nil
}

// getRemoteURL returns the URL of the "origin" remote.
func getRemoteURL(gitDir string) (string, error) {
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	cmd.Dir = gitDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("no 'origin' remote configured: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// initBranchInTempDir creates a fresh git repo in tmpDir, writes the initial
// data files, commits them, and pushes to remoteURL as BranchName.
func initBranchInTempDir(tmpDir, remoteURL string) error {
	// Initialize a new git repo.
	if err := run(tmpDir, "git", "init"); err != nil {
		return fmt.Errorf("git init in temp dir: %w", err)
	}

	// Write data files.
	if err := writeInitialFiles(tmpDir); err != nil {
		return fmt.Errorf("writing initial files: %w", err)
	}

	// Stage everything.
	if err := run(tmpDir, "git", "add", "."); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	// Commit with --no-verify (data-only, pre-commit hooks don't apply).
	if err := run(tmpDir, "git", "commit", "--no-verify", "-m", "Initialize draft review data"); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	// Add origin remote.
	if err := run(tmpDir, "git", "remote", "add", "origin", remoteURL); err != nil {
		return fmt.Errorf("git remote add: %w", err)
	}

	// Push to origin as the review branch.
	if err := run(tmpDir, "git", "push", "origin", "HEAD:refs/heads/"+BranchName); err != nil {
		return fmt.Errorf("pushing branch to origin: %w", err)
	}

	return nil
}

// writeInitialFiles creates the schema-version file, config.json, and empty
// subdirectories (with .gitkeep files) in the given directory.
func writeInitialFiles(dir string) error {
	// schema-version
	if err := os.WriteFile(filepath.Join(dir, "schema-version"), []byte(SchemaVersion), 0644); err != nil {
		return err
	}

	// config.json
	cfg := DefaultConfig()
	data, err := cfg.MarshalJSON()
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), append(data, '\n'), 0644); err != nil {
		return err
	}

	// Empty directories with .gitkeep
	for _, sub := range []string{"threads", "reviews", "participants"} {
		subDir := filepath.Join(dir, sub)
		if err := os.MkdirAll(subDir, 0755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(subDir, ".gitkeep"), []byte(""), 0644); err != nil {
			return err
		}
	}

	return nil
}

// fetchBranch fetches the review branch from origin into the local repo.
func fetchBranch(gitDir string) error {
	return run(gitDir, "git", "fetch", "origin", BranchName+":"+BranchName)
}

// run executes a command in the given directory, combining stdout and stderr
// for error reporting.
func run(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, string(out))
	}
	return nil
}
