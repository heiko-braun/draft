package review

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupRepoWithDocs creates a bare remote + working clone that has
// document directories populated (specs/, docs/) and a review branch.
// Returns the working directory.
func setupRepoWithDocs(t *testing.T) string {
	t.Helper()

	workDir := setupBareRemote(t)

	// Create document directories with some files.
	writeTestFile(t, workDir, "specs/auth.md", "# Auth Spec\n\nAuthentication design.\n")
	writeTestFile(t, workDir, "docs/readme.md", "# Docs\n\nProject documentation.\n")
	runGit(t, workDir, "git", "add", ".")
	runGit(t, workDir, "git", "commit", "--no-verify", "-m", "add documents")
	runGit(t, workDir, "git", "push", "origin", "HEAD")

	// Initialize the review branch so it exists for the reviews worktree.
	if err := InitReviewBranch(workDir); err != nil {
		t.Fatalf("InitReviewBranch failed: %v", err)
	}

	return workDir
}

func TestEnsureWorktrees_Creates(t *testing.T) {
	workDir := setupRepoWithDocs(t)

	// Override home directory so worktrees go to a temp location.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cfg := DefaultConfig()
	result, err := EnsureWorktrees(workDir, cfg, "main")
	if err != nil {
		t.Fatalf("EnsureWorktrees failed: %v", err)
	}

	// Verify DocsPath exists and is a git directory.
	if _, err := os.Stat(result.DocsPath); os.IsNotExist(err) {
		t.Fatal("DocsPath does not exist")
	}
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = result.DocsPath
	if err := cmd.Run(); err != nil {
		t.Fatal("DocsPath is not a valid git directory")
	}

	// Verify ReviewsPath exists and is a git directory.
	if _, err := os.Stat(result.ReviewsPath); os.IsNotExist(err) {
		t.Fatal("ReviewsPath does not exist")
	}
	cmd = exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = result.ReviewsPath
	if err := cmd.Run(); err != nil {
		t.Fatal("ReviewsPath is not a valid git directory")
	}

	// Verify RepoID is set.
	if len(result.RepoID) != 12 {
		t.Errorf("RepoID length = %d, want 12", len(result.RepoID))
	}
}

func TestEnsureWorktrees_SparseCheckout(t *testing.T) {
	workDir := setupRepoWithDocs(t)

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cfg := ReviewConfig{
		DocumentPaths: []string{"specs/"},
		FilePatterns:  []string{"*.md"},
		DefaultBranch: "main",
	}

	result, err := EnsureWorktrees(workDir, cfg, "main")
	if err != nil {
		t.Fatalf("EnsureWorktrees failed: %v", err)
	}

	// Verify specs/ directory exists in the docs worktree.
	specsDir := filepath.Join(result.DocsPath, "specs")
	if _, err := os.Stat(specsDir); os.IsNotExist(err) {
		t.Error("specs/ directory should exist in sparse checkout")
	}

	// Verify auth.md exists in specs/.
	authFile := filepath.Join(result.DocsPath, "specs", "auth.md")
	if _, err := os.Stat(authFile); os.IsNotExist(err) {
		t.Error("specs/auth.md should exist in sparse checkout")
	}
}

func TestEnsureWorktrees_ReviewsWorktreeOnBranch(t *testing.T) {
	workDir := setupRepoWithDocs(t)

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cfg := DefaultConfig()
	result, err := EnsureWorktrees(workDir, cfg, "main")
	if err != nil {
		t.Fatalf("EnsureWorktrees failed: %v", err)
	}

	// Verify the reviews worktree is on the draft/reviews branch.
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = result.ReviewsPath
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git branch --show-current failed: %v", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch != BranchName {
		t.Errorf("reviews worktree branch = %q, want %q", branch, BranchName)
	}

	// Verify review data files exist.
	schemaFile := filepath.Join(result.ReviewsPath, "schema-version")
	if _, err := os.Stat(schemaFile); os.IsNotExist(err) {
		t.Error("schema-version should exist in reviews worktree")
	}
}

func TestEnsureWorktrees_Idempotent(t *testing.T) {
	workDir := setupRepoWithDocs(t)

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cfg := DefaultConfig()

	// First call creates worktrees.
	result1, err := EnsureWorktrees(workDir, cfg, "main")
	if err != nil {
		t.Fatalf("first EnsureWorktrees failed: %v", err)
	}

	// Second call should succeed and return the same paths.
	result2, err := EnsureWorktrees(workDir, cfg, "main")
	if err != nil {
		t.Fatalf("second EnsureWorktrees failed: %v", err)
	}

	if result1.DocsPath != result2.DocsPath {
		t.Errorf("DocsPath changed: %q -> %q", result1.DocsPath, result2.DocsPath)
	}
	if result1.ReviewsPath != result2.ReviewsPath {
		t.Errorf("ReviewsPath changed: %q -> %q", result1.ReviewsPath, result2.ReviewsPath)
	}
}

func TestEnsureWorktrees_BrokenWorktreeRecreated(t *testing.T) {
	workDir := setupRepoWithDocs(t)

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cfg := DefaultConfig()

	// Create worktrees.
	result, err := EnsureWorktrees(workDir, cfg, "main")
	if err != nil {
		t.Fatalf("EnsureWorktrees failed: %v", err)
	}

	// Corrupt the docs worktree by removing its .git file.
	gitFile := filepath.Join(result.DocsPath, ".git")
	os.Remove(gitFile)

	// Prune stale worktree entries so git doesn't complain.
	runGit(t, workDir, "git", "worktree", "prune")

	// Re-ensure should detect the broken worktree and re-create it.
	result2, err := EnsureWorktrees(workDir, cfg, "main")
	if err != nil {
		t.Fatalf("EnsureWorktrees after corruption failed: %v", err)
	}

	// Verify the recreated docs worktree is valid.
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = result2.DocsPath
	if err := cmd.Run(); err != nil {
		t.Fatal("re-created DocsPath is not a valid git directory")
	}
}

func TestVerifyWorktree_NonexistentPath(t *testing.T) {
	workDir := setupBareRemote(t)

	valid := verifyWorktree(workDir, "/nonexistent/path")
	if valid {
		t.Error("verifyWorktree should return false for nonexistent path")
	}
}

func TestListWorktrees(t *testing.T) {
	workDir := setupBareRemote(t)

	paths, err := listWorktrees(workDir)
	if err != nil {
		t.Fatalf("listWorktrees failed: %v", err)
	}

	// The main working directory should always be listed.
	if len(paths) == 0 {
		t.Fatal("listWorktrees returned no paths")
	}

	// The first entry should be the repo itself (resolve symlinks for macOS).
	resolvedWorkDir, _ := filepath.EvalSymlinks(workDir)
	found := false
	for _, p := range paths {
		resolvedP, _ := filepath.EvalSymlinks(p)
		if resolvedP == resolvedWorkDir {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("listWorktrees should include %q, got %v", workDir, paths)
	}
}

func TestLiveModifications_DetectsChanges(t *testing.T) {
	workDir := setupRepoWithDocs(t)

	// Modify a tracked document file.
	specFile := filepath.Join(workDir, "specs", "auth.md")
	if err := os.WriteFile(specFile, []byte("# Modified\n"), 0644); err != nil {
		t.Fatal(err)
	}

	modified, err := LiveModifications(workDir, []string{"specs/", "docs/"})
	if err != nil {
		t.Fatalf("LiveModifications failed: %v", err)
	}

	if len(modified) == 0 {
		t.Fatal("LiveModifications should detect the modified file")
	}

	found := false
	for _, m := range modified {
		if m == "specs/auth.md" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("LiveModifications should include specs/auth.md, got %v", modified)
	}
}

func TestLiveModifications_IgnoresNonDocPaths(t *testing.T) {
	workDir := setupRepoWithDocs(t)

	// Create and modify a file outside document paths.
	writeTestFile(t, workDir, "src/main.go", "package main\n")
	runGit(t, workDir, "git", "add", "src/main.go")
	runGit(t, workDir, "git", "commit", "--no-verify", "-m", "add source")
	runGit(t, workDir, "git", "push", "origin", "HEAD")

	// Modify the source file.
	if err := os.WriteFile(filepath.Join(workDir, "src", "main.go"), []byte("package main // modified\n"), 0644); err != nil {
		t.Fatal(err)
	}

	modified, err := LiveModifications(workDir, []string{"specs/", "docs/"})
	if err != nil {
		t.Fatalf("LiveModifications failed: %v", err)
	}

	for _, m := range modified {
		if strings.HasPrefix(m, "src/") {
			t.Errorf("LiveModifications should not include files outside doc paths, got %q", m)
		}
	}
}

func TestLiveModifications_NoChanges(t *testing.T) {
	workDir := setupRepoWithDocs(t)

	modified, err := LiveModifications(workDir, []string{"specs/", "docs/"})
	if err != nil {
		t.Fatalf("LiveModifications failed: %v", err)
	}

	if len(modified) != 0 {
		t.Errorf("LiveModifications should return empty when no changes, got %v", modified)
	}
}

func TestLiveModifications_UntrackedFile(t *testing.T) {
	workDir := setupRepoWithDocs(t)

	// Add a new untracked file in a document path.
	writeTestFile(t, workDir, "specs/new-spec.md", "# New Spec\n")

	modified, err := LiveModifications(workDir, []string{"specs/"})
	if err != nil {
		t.Fatalf("LiveModifications failed: %v", err)
	}

	found := false
	for _, m := range modified {
		if m == "specs/new-spec.md" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("LiveModifications should detect untracked files in doc paths, got %v", modified)
	}
}

func TestIsUnderDocPaths(t *testing.T) {
	docPaths := []string{"specs/", "docs/", "rfcs/"}

	tests := []struct {
		filePath string
		want     bool
	}{
		{"specs/auth.md", true},
		{"docs/readme.md", true},
		{"rfcs/001.md", true},
		{"src/main.go", false},
		{"README.md", false},
		{"specs-old/foo.md", false},
		{"specs/nested/deep.md", true},
	}

	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			got := isUnderDocPaths(tt.filePath, docPaths)
			if got != tt.want {
				t.Errorf("isUnderDocPaths(%q) = %v, want %v", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestEnsureWorktrees_DoesNotDisturbWorkingTree(t *testing.T) {
	workDir := setupRepoWithDocs(t)

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create an untracked file that must survive.
	untrackedPath := filepath.Join(workDir, "untracked.txt")
	if err := os.WriteFile(untrackedPath, []byte("important data"), 0644); err != nil {
		t.Fatal(err)
	}

	// Record the current branch.
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = workDir
	branchBefore, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	if _, err := EnsureWorktrees(workDir, cfg, "main"); err != nil {
		t.Fatalf("EnsureWorktrees failed: %v", err)
	}

	// Verify the untracked file still exists.
	if _, err := os.Stat(untrackedPath); os.IsNotExist(err) {
		t.Fatal("untracked file was destroyed")
	}
	data, err := os.ReadFile(untrackedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "important data" {
		t.Errorf("untracked file content = %q, want %q", string(data), "important data")
	}

	// Verify we are still on the same branch.
	cmd = exec.Command("git", "branch", "--show-current")
	cmd.Dir = workDir
	branchAfter, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(branchBefore)) != strings.TrimSpace(string(branchAfter)) {
		t.Errorf("branch changed from %q to %q", strings.TrimSpace(string(branchBefore)), strings.TrimSpace(string(branchAfter)))
	}
}
