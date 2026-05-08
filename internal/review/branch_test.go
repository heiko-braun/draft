package review

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupBareRemote creates a bare git repo that acts as the "origin" remote,
// and a cloned working repo that points to it. Returns (workingDir, cleanup).
func setupBareRemote(t *testing.T) string {
	t.Helper()

	base := t.TempDir()
	bareDir := filepath.Join(base, "remote.git")
	workDir := filepath.Join(base, "work")

	// Create bare repo.
	runGit(t, "", "git", "init", "--bare", bareDir)

	// Clone it to get a working repo with origin configured.
	runGit(t, "", "git", "clone", bareDir, workDir)

	// Git needs at least one commit to have a valid HEAD; create an initial commit.
	writeTestFile(t, workDir, "README.md", "# test\n")
	runGit(t, workDir, "git", "add", ".")
	runGit(t, workDir, "git", "commit", "--no-verify", "-m", "initial commit")
	runGit(t, workDir, "git", "push", "origin", "HEAD")

	return workDir
}

func runGit(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestInitReviewBranch_CreatesExpectedStructure(t *testing.T) {
	workDir := setupBareRemote(t)

	if err := InitReviewBranch(workDir); err != nil {
		t.Fatalf("InitReviewBranch failed: %v", err)
	}

	// Verify the local branch exists.
	cmd := exec.Command("git", "rev-parse", "--verify", "refs/heads/"+BranchName)
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("branch %q not found locally: %v", BranchName, err)
	}

	// Checkout the branch to inspect files.
	inspectDir := t.TempDir()
	runGit(t, "", "git", "clone", "--branch", BranchName, workDir, inspectDir)

	// Verify schema-version.
	data, err := os.ReadFile(filepath.Join(inspectDir, "schema-version"))
	if err != nil {
		t.Fatalf("schema-version not found: %v", err)
	}
	if string(data) != SchemaVersion {
		t.Errorf("schema-version = %q, want %q", string(data), SchemaVersion)
	}

	// Verify config.json.
	data, err = os.ReadFile(filepath.Join(inspectDir, "config.json"))
	if err != nil {
		t.Fatalf("config.json not found: %v", err)
	}
	var cfg ReviewConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("config.json is invalid JSON: %v", err)
	}
	if cfg.DefaultBranch != "main" {
		t.Errorf("config.json default_branch = %q, want %q", cfg.DefaultBranch, "main")
	}
	if len(cfg.DocumentPaths) != 4 {
		t.Errorf("config.json document_paths length = %d, want 4", len(cfg.DocumentPaths))
	}

	// Verify empty directories (via .gitkeep).
	for _, sub := range []string{"threads", "reviews", "participants"} {
		keepFile := filepath.Join(inspectDir, sub, ".gitkeep")
		if _, err := os.Stat(keepFile); os.IsNotExist(err) {
			t.Errorf("%s/.gitkeep not found", sub)
		}
	}
}

func TestInitReviewBranch_Idempotent(t *testing.T) {
	workDir := setupBareRemote(t)

	// First call creates the branch.
	if err := InitReviewBranch(workDir); err != nil {
		t.Fatalf("first InitReviewBranch failed: %v", err)
	}

	// Second call should be a no-op (no error).
	if err := InitReviewBranch(workDir); err != nil {
		t.Fatalf("second InitReviewBranch failed: %v", err)
	}

	// Verify only one commit on the branch (no duplicate commits).
	cmd := exec.Command("git", "rev-list", "--count", BranchName)
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-list failed: %v", err)
	}
	count := strings.TrimSpace(string(out))
	if count != "1" {
		t.Errorf("expected 1 commit on %s, got %s", BranchName, count)
	}
}

func TestInitReviewBranch_DoesNotDisturbWorkingTree(t *testing.T) {
	workDir := setupBareRemote(t)

	// Create an untracked file that must survive.
	untrackedPath := filepath.Join(workDir, "untracked.txt")
	if err := os.WriteFile(untrackedPath, []byte("important data"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := InitReviewBranch(workDir); err != nil {
		t.Fatalf("InitReviewBranch failed: %v", err)
	}

	// Verify the untracked file is still present.
	if _, err := os.Stat(untrackedPath); os.IsNotExist(err) {
		t.Fatal("untracked file was destroyed by InitReviewBranch")
	}
	data, err := os.ReadFile(untrackedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "important data" {
		t.Errorf("untracked file content = %q, want %q", string(data), "important data")
	}

	// Verify we are still on the original branch (not switched to review branch).
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git branch --show-current failed: %v", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch == BranchName {
		t.Error("working tree was switched to the review branch")
	}
}

func TestBranchExists_DetectsLocalBranch(t *testing.T) {
	workDir := setupBareRemote(t)

	// Initially no review branch.
	exists, err := branchExists(workDir)
	if err != nil {
		t.Fatalf("branchExists failed: %v", err)
	}
	if exists {
		t.Error("branchExists returned true before branch creation")
	}

	// Create the branch.
	if err := InitReviewBranch(workDir); err != nil {
		t.Fatalf("InitReviewBranch failed: %v", err)
	}

	// Now it should exist.
	exists, err = branchExists(workDir)
	if err != nil {
		t.Fatalf("branchExists failed: %v", err)
	}
	if !exists {
		t.Error("branchExists returned false after branch creation")
	}
}

func TestBranchExists_DetectsRemoteBranch(t *testing.T) {
	workDir := setupBareRemote(t)

	// Create the branch and push it.
	if err := InitReviewBranch(workDir); err != nil {
		t.Fatalf("InitReviewBranch failed: %v", err)
	}

	// Delete the local branch ref but keep the remote.
	runGit(t, workDir, "git", "branch", "-D", BranchName)

	// branchExists should still detect it via ls-remote.
	exists, err := branchExists(workDir)
	if err != nil {
		t.Fatalf("branchExists failed: %v", err)
	}
	if !exists {
		t.Error("branchExists should detect branch on remote")
	}
}

func TestWriteInitialFiles(t *testing.T) {
	dir := t.TempDir()

	if err := writeInitialFiles(dir); err != nil {
		t.Fatalf("writeInitialFiles failed: %v", err)
	}

	// schema-version
	data, err := os.ReadFile(filepath.Join(dir, "schema-version"))
	if err != nil {
		t.Fatalf("schema-version: %v", err)
	}
	if string(data) != SchemaVersion {
		t.Errorf("schema-version = %q, want %q", string(data), SchemaVersion)
	}

	// config.json
	data, err = os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("config.json: %v", err)
	}
	if !json.Valid(data) {
		t.Error("config.json is not valid JSON")
	}

	// Directories with .gitkeep
	for _, sub := range []string{"threads", "reviews", "participants"} {
		info, err := os.Stat(filepath.Join(dir, sub))
		if err != nil {
			t.Errorf("%s directory: %v", sub, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", sub)
		}
		if _, err := os.Stat(filepath.Join(dir, sub, ".gitkeep")); os.IsNotExist(err) {
			t.Errorf("%s/.gitkeep not found", sub)
		}
	}
}
