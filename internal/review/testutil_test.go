package review

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// writeTestFile creates a file at root/relPath with the given content.
func writeTestFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	abs := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(abs), err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", abs, err)
	}
}

// setupBareRemote creates a bare git remote and a clone with an initial commit.
// Returns the path to the working clone.
func setupBareRemote(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	bare := filepath.Join(dir, "remote.git")
	work := filepath.Join(dir, "work")

	run := func(d string, args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = d
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	run(dir, "git", "init", "--bare", bare)
	run(dir, "git", "clone", bare, work)
	run(work, "git", "config", "user.email", "test@example.com")
	run(work, "git", "config", "user.name", "Test")

	writeTestFile(t, work, "README.md", "# Test\n")
	run(work, "git", "add", ".")
	run(work, "git", "commit", "-m", "init")
	run(work, "git", "push", "origin", "HEAD")

	return work
}
