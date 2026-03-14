package search

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupProject(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	// Create project files.
	writeFile(t, root, "main.go", "package main\nfunc main() {}\n")
	writeFile(t, root, "README.md", "# Test Project\nThis is a test.\n")
	writeFile(t, root, "config.yaml", "key: value\n")

	return root, dbPath
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestIndex_FullIndex(t *testing.T) {
	root, dbPath := setupProject(t)

	s, err := OpenStore(dbPath, root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	result, err := Index(s, root, false)
	if err != nil {
		t.Fatal(err)
	}

	if result.FilesIndexed != 3 {
		t.Errorf("FilesIndexed = %d, want 3", result.FilesIndexed)
	}

	count, _ := s.FileCount()
	if count != 3 {
		t.Errorf("FileCount = %d, want 3", count)
	}
}

func TestIndex_GitignoreRespected(t *testing.T) {
	root, dbPath := setupProject(t)

	// Add .gitignore and ignored file.
	writeFile(t, root, ".gitignore", "ignored/\n*.log\n")
	writeFile(t, root, "ignored/secret.go", "package secret\n")
	writeFile(t, root, "debug.log", "some log\n")

	s, err := OpenStore(dbPath, root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	Index(s, root, false)

	// ignored/secret.go and debug.log should not be indexed.
	f1, _ := s.GetFile("ignored/secret.go")
	if f1 != nil {
		t.Error("gitignored directory file should not be indexed")
	}
	f2, _ := s.GetFile("debug.log")
	if f2 != nil {
		t.Error("gitignored file should not be indexed")
	}

	// .gitignore itself should be indexed.
	f3, _ := s.GetFile(".gitignore")
	if f3 == nil {
		t.Error(".gitignore should be indexed")
	}
}

func TestIndex_BinarySkipped(t *testing.T) {
	root, dbPath := setupProject(t)

	// Write a binary file (contains null bytes).
	binContent := []byte("header\x00\x00\x00binary data")
	os.WriteFile(filepath.Join(root, "image.png"), binContent, 0644)

	s, err := OpenStore(dbPath, root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	result, err := Index(s, root, false)
	if err != nil {
		t.Fatal(err)
	}

	f, _ := s.GetFile("image.png")
	if f != nil {
		t.Error("binary file should not be indexed")
	}
	if result.FilesSkipped < 1 {
		t.Error("expected at least 1 skipped file")
	}
}

func TestIndex_LargeFileSkipped(t *testing.T) {
	root, dbPath := setupProject(t)

	// Write a file > 1MB.
	large := make([]byte, maxFileSize+1)
	for i := range large {
		large[i] = 'a'
	}
	os.WriteFile(filepath.Join(root, "huge.txt"), large, 0644)

	s, err := OpenStore(dbPath, root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	result, err := Index(s, root, false)
	if err != nil {
		t.Fatal(err)
	}

	f, _ := s.GetFile("huge.txt")
	if f != nil {
		t.Error("large file should not be indexed")
	}
	if result.FilesSkipped < 1 {
		t.Error("expected at least 1 skipped file")
	}
}

func TestIndex_IncrementalUpdate(t *testing.T) {
	root, dbPath := setupProject(t)

	s, err := OpenStore(dbPath, root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// First index.
	Index(s, root, false)

	// Record indexed timestamp for main.go.
	f1, _ := s.GetFile("main.go")
	origIndexed := f1.Indexed

	// Modify main.go and set a future mtime to ensure mtime differs.
	writeFile(t, root, "main.go", "package main\nfunc main() { fmt.Println(\"updated\") }\n")
	future := time.Now().Add(2 * time.Hour)
	os.Chtimes(filepath.Join(root, "main.go"), future, future)

	// Re-index.
	result, err := Index(s, root, false)
	if err != nil {
		t.Fatal(err)
	}

	if result.FilesIndexed != 1 {
		t.Errorf("FilesIndexed = %d, want 1", result.FilesIndexed)
	}

	// main.go should have a newer indexed timestamp.
	f2, _ := s.GetFile("main.go")
	if f2.Indexed < origIndexed {
		t.Errorf("indexed timestamp should have been updated: got %d, orig %d", f2.Indexed, origIndexed)
	}
	// Hash should have changed.
	if f2.Hash == f1.Hash {
		t.Error("hash should have changed after content update")
	}
}

func TestIndex_IncrementalDelete(t *testing.T) {
	root, dbPath := setupProject(t)

	s, err := OpenStore(dbPath, root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	Index(s, root, false)

	// Delete README.md.
	os.Remove(filepath.Join(root, "README.md"))

	result, err := Index(s, root, false)
	if err != nil {
		t.Fatal(err)
	}

	if result.FilesDeleted != 1 {
		t.Errorf("FilesDeleted = %d, want 1", result.FilesDeleted)
	}

	f, _ := s.GetFile("README.md")
	if f != nil {
		t.Error("deleted file should not be in index")
	}

	count, _ := s.FileCount()
	if count != 2 {
		t.Errorf("FileCount = %d, want 2", count)
	}
}

func TestIndex_MtimeOnlyNoReindex(t *testing.T) {
	root, dbPath := setupProject(t)

	s, err := OpenStore(dbPath, root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	Index(s, root, false)

	// Touch main.go (change mtime but not content).
	now := time.Now().Add(time.Hour)
	os.Chtimes(filepath.Join(root, "main.go"), now, now)

	result, err := Index(s, root, false)
	if err != nil {
		t.Fatal(err)
	}

	// Should be counted as unchanged (mtime updated, no FTS re-insert).
	if result.FilesIndexed != 0 {
		t.Errorf("FilesIndexed = %d, want 0 (mtime-only change)", result.FilesIndexed)
	}
}

func TestIndex_ForceRebuild(t *testing.T) {
	root, dbPath := setupProject(t)

	s, err := OpenStore(dbPath, root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	Index(s, root, false)
	time.Sleep(10 * time.Millisecond)

	// Force rebuild.
	result, err := Index(s, root, true)
	if err != nil {
		t.Fatal(err)
	}

	// All files should be re-indexed.
	if result.FilesIndexed != 3 {
		t.Errorf("FilesIndexed = %d, want 3 after force rebuild", result.FilesIndexed)
	}
}

func TestIndex_SkipsDotGit(t *testing.T) {
	root, dbPath := setupProject(t)

	// Create .git directory with files.
	writeFile(t, root, ".git/config", "[core]\nbare = false\n")

	s, err := OpenStore(dbPath, root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	Index(s, root, false)

	f, _ := s.GetFile(".git/config")
	if f != nil {
		t.Error(".git directory should be skipped")
	}
}

func TestPruneIndexes(t *testing.T) {
	// Create two project directories.
	proj1 := t.TempDir()
	proj2 := t.TempDir()

	cacheDir := t.TempDir()
	db1 := filepath.Join(cacheDir, "proj1.db")
	db2 := filepath.Join(cacheDir, "proj2.db")

	// Index both.
	s1, _ := OpenStore(db1, proj1)
	writeFile(t, proj1, "a.go", "package a")
	Index(s1, proj1, false)
	s1.Close()

	s2, _ := OpenStore(db2, proj2)
	writeFile(t, proj2, "b.go", "package b")
	Index(s2, proj2, false)
	s2.Close()

	// Delete proj2's directory.
	os.RemoveAll(proj2)

	// PruneIndexes works on the default cache dir, so we test readProjectRoot directly.
	root, err := readProjectRoot(db2)
	if err != nil {
		t.Fatal(err)
	}
	if root != proj2 {
		t.Errorf("got %q, want %q", root, proj2)
	}
}

func TestListIndexes(t *testing.T) {
	proj := t.TempDir()
	cacheDir := t.TempDir()
	db := filepath.Join(cacheDir, "test.db")

	s, _ := OpenStore(db, proj)
	writeFile(t, proj, "main.go", "package main")
	Index(s, proj, false)
	s.Close()

	info, err := readIndexInfo(db)
	if err != nil {
		t.Fatal(err)
	}

	if info.ProjectRoot != proj {
		t.Errorf("ProjectRoot = %q, want %q", info.ProjectRoot, proj)
	}
	if info.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", info.FileCount)
	}
	if info.SizeBytes == 0 {
		t.Error("SizeBytes should be > 0")
	}
}
