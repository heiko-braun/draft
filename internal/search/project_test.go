package search

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectProjectRoot_WithDraftMarker(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".draft"), 0755); err != nil {
		t.Fatal(err)
	}

	got, err := DetectProjectRoot(sub)
	if err != nil {
		t.Fatal(err)
	}

	// Resolve symlinks on root for comparison (macOS /tmp → /private/tmp).
	want, _ := filepath.EvalSymlinks(root)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDetectProjectRoot_WithClaudeMD(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "src")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := DetectProjectRoot(sub)
	if err != nil {
		t.Fatal(err)
	}

	want, _ := filepath.EvalSymlinks(root)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDetectProjectRoot_FallbackToCwd(t *testing.T) {
	dir := t.TempDir()

	got, err := DetectProjectRoot(dir)
	if err != nil {
		t.Fatal(err)
	}

	want, _ := filepath.EvalSymlinks(dir)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestIndexPath_Deterministic(t *testing.T) {
	dir := t.TempDir()

	p1, err := IndexPath(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	p2, err := IndexPath(dir, "")
	if err != nil {
		t.Fatal(err)
	}

	if p1 != p2 {
		t.Errorf("not deterministic: %q vs %q", p1, p2)
	}
	if filepath.Ext(p1) != ".db" {
		t.Errorf("expected .db extension, got %q", p1)
	}
}

func TestIndexPath_Override(t *testing.T) {
	override := "/tmp/test-custom.db"
	got, err := IndexPath("/some/project", override)
	if err != nil {
		t.Fatal(err)
	}
	if got != override {
		t.Errorf("got %q, want %q", got, override)
	}
}

func TestIndexPath_SymlinkResolution(t *testing.T) {
	real := t.TempDir()
	parent := t.TempDir()
	link := filepath.Join(parent, "link")

	if err := os.Symlink(real, link); err != nil {
		t.Skip("symlinks not supported")
	}

	p1, err := IndexPath(real, "")
	if err != nil {
		t.Fatal(err)
	}
	p2, err := IndexPath(link, "")
	if err != nil {
		t.Fatal(err)
	}

	if p1 != p2 {
		t.Errorf("symlink produced different path: %q vs %q", p1, p2)
	}
}
