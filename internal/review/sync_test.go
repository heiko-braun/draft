package review

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSyncer_HasPendingChanges_NoChanges(t *testing.T) {
	// Set up a minimal git repo to act as the reviews worktree.
	dir := initTestGitRepo(t)

	syncer := NewSyncer(dir, dir, dir, "main")

	has, err := syncer.HasPendingChanges()
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("HasPendingChanges should be false for clean repo")
	}
}

func TestSyncer_HasPendingChanges_WithChanges(t *testing.T) {
	dir := initTestGitRepo(t)

	// Create an untracked file.
	if err := os.WriteFile(filepath.Join(dir, "new.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	syncer := NewSyncer(dir, dir, dir, "main")

	has, err := syncer.HasPendingChanges()
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Error("HasPendingChanges should be true for dirty repo")
	}
}

func TestSyncer_Publish_NothingToCommit(t *testing.T) {
	dir := initTestGitRepoWithRemote(t)

	syncer := NewSyncer(dir, dir, dir, "main")

	// Publish with no changes should be a no-op.
	if err := syncer.Publish(); err != nil {
		t.Fatalf("Publish with nothing to commit: %v", err)
	}
}

func TestSyncer_Publish_CommitsAndPushes(t *testing.T) {
	dir := initTestGitRepoWithRemote(t)

	// Create a file to commit.
	reviewsDir := filepath.Join(dir, "reviews")
	if err := os.MkdirAll(reviewsDir, 0755); err != nil {
		t.Fatal(err)
	}
	reviewFile := filepath.Join(reviewsDir, "test-review.json")
	r := Review{
		ID:        "test-review",
		Title:     "Publish Test",
		Status:    ReviewOpen,
		Documents: []string{"doc.md"},
		CreatedAt: "2024-01-01T00:00:00Z",
		UpdatedAt: "2024-01-01T00:00:00Z",
	}
	data, _ := json.MarshalIndent(r, "", "  ")
	if err := os.WriteFile(reviewFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	syncer := NewSyncer(dir, dir, dir, "main")

	if err := syncer.Publish(); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Verify the file was committed.
	cmd := exec.Command("git", "log", "--oneline", "-1")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Error("expected a commit after publish")
	}

	// Verify no pending changes after publish.
	has, err := syncer.HasPendingChanges()
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("should have no pending changes after publish")
	}
}

func TestIsThreadFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"threads/doc.md/thread-1.json", true},
		{"threads/docs/nested/file.md/abc.json", true},
		{"reviews/review-1.json", false},
		{"participants/user.json", false},
		{"threads/doc.md/thread-1.txt", false},
		{"otherthreads/doc.md/t.json", false},
	}

	for _, tt := range tests {
		got := isThreadFile(tt.path)
		if got != tt.want {
			t.Errorf("isThreadFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestSemanticMergeThread_Integration(t *testing.T) {
	// Simulate a semantic merge by writing two thread versions and merging.
	ours := Thread{
		ID:        "t-merge",
		Document:  "doc.md",
		Status:    ThreadOpen,
		UpdatedAt: "2024-01-01T10:00:00Z",
		Comments: []Comment{
			{ID: "c-1", Author: "alice", Body: "Comment 1", CreatedAt: "2024-01-01T09:00:00Z"},
			{ID: "c-2", Author: "alice", Body: "Comment 2", CreatedAt: "2024-01-01T09:30:00Z"},
		},
	}

	theirs := Thread{
		ID:        "t-merge",
		Document:  "doc.md",
		Status:    ThreadResolved,
		UpdatedAt: "2024-01-01T12:00:00Z",
		Comments: []Comment{
			{ID: "c-1", Author: "alice", Body: "Comment 1", CreatedAt: "2024-01-01T09:00:00Z"},
			{ID: "c-3", Author: "bob", Body: "Comment 3", CreatedAt: "2024-01-01T11:00:00Z"},
		},
	}

	merged := MergeThreads(&ours, &theirs)

	// Should have 3 unique comments.
	if len(merged.Comments) != 3 {
		t.Fatalf("merged comments = %d, want 3", len(merged.Comments))
	}

	// Should be ordered by CreatedAt.
	expectedOrder := []string{"c-1", "c-2", "c-3"}
	for i, id := range expectedOrder {
		if merged.Comments[i].ID != id {
			t.Errorf("comment[%d] = %q, want %q", i, merged.Comments[i].ID, id)
		}
	}

	// Status should be from theirs (later UpdatedAt).
	if merged.Status != ThreadResolved {
		t.Errorf("Status = %q, want %q", merged.Status, ThreadResolved)
	}
}

func TestMergeThreads_EmptyComments(t *testing.T) {
	ours := &Thread{
		ID:        "t-1",
		Document:  "doc.md",
		Status:    ThreadOpen,
		UpdatedAt: "2024-01-01T10:00:00Z",
	}
	theirs := &Thread{
		ID:        "t-1",
		Document:  "doc.md",
		Status:    ThreadOpen,
		UpdatedAt: "2024-01-01T10:00:00Z",
		Comments: []Comment{
			{ID: "c-1", Author: "bob", Body: "Hello", CreatedAt: "2024-01-01T09:00:00Z"},
		},
	}

	merged := MergeThreads(ours, theirs)

	if len(merged.Comments) != 1 {
		t.Errorf("merged comments = %d, want 1", len(merged.Comments))
	}
}

func TestMergeThreads_PreservesAnchor(t *testing.T) {
	anchor := Anchor{
		FileHash: "hash",
		Start:    50,
		End:      75,
		Excerpt:  "test content",
	}

	ours := &Thread{
		ID:        "t-1",
		Document:  "doc.md",
		Anchor:    anchor,
		Status:    ThreadOpen,
		UpdatedAt: "2024-01-01T10:00:00Z",
	}
	theirs := &Thread{
		ID:        "t-1",
		Document:  "doc.md",
		Anchor:    anchor,
		Status:    ThreadOpen,
		UpdatedAt: "2024-01-01T09:00:00Z",
	}

	merged := MergeThreads(ours, theirs)

	if merged.Anchor.Start != 50 {
		t.Errorf("Anchor Start = %d, want 50", merged.Anchor.Start)
	}
}

// --- Test helpers ---

// initTestGitRepo creates a temporary directory with a git repo and an initial commit.
func initTestGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	// Create initial commit.
	initFile := filepath.Join(dir, ".gitkeep")
	if err := os.WriteFile(initFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	cmds = [][]string{
		{"git", "add", "."},
		{"git", "commit", "--no-verify", "-m", "initial"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	return dir
}

// initTestGitRepoWithRemote creates a git repo with a bare remote for push testing.
func initTestGitRepoWithRemote(t *testing.T) string {
	t.Helper()

	// Create bare remote.
	bareDir := t.TempDir()
	cmd := exec.Command("git", "init", "--bare")
	cmd.Dir = bareDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare failed: %v\n%s", err, out)
	}

	// Create working repo.
	workDir := t.TempDir()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "remote", "add", "origin", bareDir},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = workDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	// Create initial commit and push.
	initFile := filepath.Join(workDir, ".gitkeep")
	if err := os.WriteFile(initFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	cmds = [][]string{
		{"git", "add", "."},
		{"git", "commit", "--no-verify", "-m", "initial"},
		{"git", "branch", "-M", BranchName},
		{"git", "push", "-u", "origin", BranchName},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = workDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	return workDir
}
