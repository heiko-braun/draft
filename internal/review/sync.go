package review

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Syncer handles fetching remote review data, publishing local changes,
// and resolving conflicts via semantic merge.
type Syncer struct {
	repoRoot     string
	reviewsPath  string
	docsPath     string
	sourceBranch string
}

// NewSyncer creates a Syncer for the given repository.
// repoRoot is the main repository root, reviewsPath is the reviews worktree,
// docsPath is the docs worktree, and sourceBranch is the branch documents are
// read from (e.g. "main").
func NewSyncer(repoRoot, reviewsPath, docsPath, sourceBranch string) *Syncer {
	return &Syncer{
		repoRoot:     repoRoot,
		reviewsPath:  reviewsPath,
		docsPath:     docsPath,
		sourceBranch: sourceBranch,
	}
}

// SyncAll fetches the latest review data and document sources from the remote.
// It fast-forwards the review worktree and updates the docs worktree.
func (s *Syncer) SyncAll() error {
	if err := s.syncReviews(); err != nil {
		return fmt.Errorf("syncing reviews: %w", err)
	}
	if err := s.syncDocs(); err != nil {
		return fmt.Errorf("syncing docs: %w", err)
	}
	return nil
}

// syncReviews fetches the review branch and fast-forwards the worktree.
func (s *Syncer) syncReviews() error {
	// Fetch the review branch from origin.
	if err := run(s.reviewsPath, "git", "fetch", "origin", BranchName); err != nil {
		return fmt.Errorf("fetching review branch: %w", err)
	}

	// Fast-forward merge.
	cmd := exec.Command("git", "merge", "--ff-only", "origin/"+BranchName)
	cmd.Dir = s.reviewsPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("fast-forwarding reviews: %w\n%s", err, string(out))
	}

	return nil
}

// syncDocs fetches and updates the document worktree to the latest source branch.
func (s *Syncer) syncDocs() error {
	if err := run(s.docsPath, "git", "fetch", "origin"); err != nil {
		return fmt.Errorf("fetching docs origin: %w", err)
	}

	ref := "origin/" + s.sourceBranch
	if err := run(s.docsPath, "git", "checkout", ref, "--"); err != nil {
		return fmt.Errorf("checking out %s: %w", ref, err)
	}

	return nil
}

// HasPendingChanges returns true if the reviews worktree has uncommitted
// changes (staged or unstaged).
func (s *Syncer) HasPendingChanges() (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = s.reviewsPath
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("checking pending changes: %w", err)
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
}

// Publish stages all changes in the reviews worktree, commits with
// --no-verify (data-only, no Go code), and pushes to origin. If the push
// is rejected due to remote changes, it fetches, rebases with semantic
// merge for thread conflicts, and retries the push.
func (s *Syncer) Publish() error {
	// Stage all changes.
	if err := run(s.reviewsPath, "git", "add", "-A"); err != nil {
		return fmt.Errorf("staging changes: %w", err)
	}

	// Check if there is anything to commit.
	hasPending, err := s.hasStagedChanges()
	if err != nil {
		return err
	}
	if !hasPending {
		return nil // nothing to publish
	}

	// Commit with --no-verify (data-only branch, pre-commit hooks don't apply).
	if err := run(s.reviewsPath, "git", "commit", "--no-verify", "-m", "Update review data"); err != nil {
		return fmt.Errorf("committing: %w", err)
	}

	// Try to push.
	if err := s.push(); err == nil {
		return nil
	}

	// Push was rejected; fetch + rebase with semantic merge.
	if err := s.fetchAndRebase(); err != nil {
		return fmt.Errorf("rebase after push rejection: %w", err)
	}

	// Retry push after successful rebase.
	if err := s.push(); err != nil {
		return fmt.Errorf("push after rebase: %w", err)
	}

	return nil
}

// hasStagedChanges checks if there are staged changes ready to commit.
func (s *Syncer) hasStagedChanges() (bool, error) {
	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	cmd.Dir = s.reviewsPath
	err := cmd.Run()
	if err != nil {
		// Exit code 1 means there are differences (staged changes).
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return true, nil
		}
		return false, fmt.Errorf("checking staged changes: %w", err)
	}
	return false, nil
}

// push pushes the review branch to origin.
func (s *Syncer) push() error {
	return run(s.reviewsPath, "git", "push", "origin", BranchName)
}

// fetchAndRebase fetches the latest remote review branch and rebases the
// local changes on top. For thread files that conflict, it performs a
// semantic merge.
func (s *Syncer) fetchAndRebase() error {
	// Fetch latest.
	if err := run(s.reviewsPath, "git", "fetch", "origin", BranchName); err != nil {
		return fmt.Errorf("fetching before rebase: %w", err)
	}

	// Attempt rebase.
	cmd := exec.Command("git", "rebase", "origin/"+BranchName)
	cmd.Dir = s.reviewsPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Rebase has conflicts; try semantic merge.
		if resolveErr := s.resolveConflicts(); resolveErr != nil {
			// Abort the rebase if we can't resolve.
			_ = run(s.reviewsPath, "git", "rebase", "--abort")
			return fmt.Errorf("resolving conflicts: %w (rebase output: %s)", resolveErr, string(out))
		}
	}

	return nil
}

// resolveConflicts iterates over conflicted files and attempts semantic
// merge for thread JSON files. Non-thread conflicts cause an error.
func (s *Syncer) resolveConflicts() error {
	conflicted, err := s.listConflictedFiles()
	if err != nil {
		return err
	}

	for _, f := range conflicted {
		if !isThreadFile(f) {
			return fmt.Errorf("non-thread conflict in %s: manual resolution required", f)
		}

		if err := s.semanticMergeThread(f); err != nil {
			return fmt.Errorf("semantic merge of %s: %w", f, err)
		}

		// Stage the resolved file.
		if err := run(s.reviewsPath, "git", "add", f); err != nil {
			return fmt.Errorf("staging resolved %s: %w", f, err)
		}
	}

	// Continue the rebase after resolving all conflicts.
	if err := run(s.reviewsPath, "git", "rebase", "--continue"); err != nil {
		// Set GIT_EDITOR to true to skip the commit message editor.
		cmd := exec.Command("git", "rebase", "--continue")
		cmd.Dir = s.reviewsPath
		cmd.Env = append(os.Environ(), "GIT_EDITOR=true")
		if out, err2 := cmd.CombinedOutput(); err2 != nil {
			return fmt.Errorf("continuing rebase: %w\n%s", err2, string(out))
		}
	}

	return nil
}

// listConflictedFiles returns the list of unmerged file paths.
func (s *Syncer) listConflictedFiles() ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	cmd.Dir = s.reviewsPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing conflicts: %w", err)
	}

	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// isThreadFile returns true if the file path is a thread JSON file.
func isThreadFile(path string) bool {
	return strings.HasPrefix(path, "threads/") && strings.HasSuffix(path, ".json")
}

// semanticMergeThread resolves a conflicted thread file by reading both
// the "ours" and "theirs" versions, merging them semantically, and writing
// the result to disk.
func (s *Syncer) semanticMergeThread(relPath string) error {
	absPath := filepath.Join(s.reviewsPath, relPath)

	oursData, err := gitShowConflictVersion(s.reviewsPath, relPath, "HEAD")
	if err != nil {
		return fmt.Errorf("reading ours version: %w", err)
	}

	theirsData, err := gitShowConflictVersion(s.reviewsPath, relPath, "MERGE_HEAD")
	if err != nil {
		// During rebase, the "theirs" is actually the rebased-onto commit.
		theirsData, err = gitShowConflictVersion(s.reviewsPath, relPath, "REBASE_HEAD")
		if err != nil {
			return fmt.Errorf("reading theirs version: %w", err)
		}
	}

	var ours, theirs Thread
	if err := json.Unmarshal(oursData, &ours); err != nil {
		return fmt.Errorf("parsing ours thread: %w", err)
	}
	if err := json.Unmarshal(theirsData, &theirs); err != nil {
		return fmt.Errorf("parsing theirs thread: %w", err)
	}

	merged := MergeThreads(&ours, &theirs)

	return writeJSON(absPath, merged)
}

// gitShowConflictVersion retrieves the content of a file at a specific
// git ref during a merge or rebase conflict.
func gitShowConflictVersion(dir, relPath, ref string) ([]byte, error) {
	cmd := exec.Command("git", "show", ref+":"+relPath)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git show %s:%s: %w", ref, relPath, err)
	}
	return out, nil
}
