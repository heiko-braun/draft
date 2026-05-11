package review

import (
	"path/filepath"
	"testing"
)

func TestNormalizeRemoteURL_SSH(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"git@github.com:user/repo.git", "github.com/user/repo"},
		{"git@github.com:user/repo", "github.com/user/repo"},
		{"git@GitHub.COM:Org/Repo.git", "github.com/Org/Repo"},
		{"git@gitlab.example.com:team/project.git", "gitlab.example.com/team/project"},
		{"deploy@bitbucket.org:company/service.git", "bitbucket.org/company/service"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeRemoteURL(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeRemoteURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeRemoteURL_HTTPS(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://github.com/user/repo.git", "github.com/user/repo"},
		{"https://github.com/user/repo", "github.com/user/repo"},
		{"https://GitHub.COM/Org/Repo.git", "github.com/Org/Repo"},
		{"http://gitlab.example.com/team/project.git", "gitlab.example.com/team/project"},
		{"https://token@github.com/user/repo.git", "github.com/user/repo"},
		{"https://user:pass@github.com/org/repo.git", "github.com/org/repo"},
		{"https://github.com/user/repo/", "github.com/user/repo"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeRemoteURL(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeRemoteURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeRemoteURL_Consistent(t *testing.T) {
	// SSH and HTTPS to the same repo should normalize to the same value.
	ssh := NormalizeRemoteURL("git@github.com:acme/platform.git")
	https := NormalizeRemoteURL("https://github.com/acme/platform.git")

	if ssh != https {
		t.Errorf("SSH %q and HTTPS %q should normalize to the same value", ssh, https)
	}
}

func TestDeriveRepoID_Deterministic(t *testing.T) {
	url := "github.com/user/repo"
	id1 := deriveRepoID(url)
	id2 := deriveRepoID(url)

	if id1 != id2 {
		t.Errorf("deriveRepoID not deterministic: %q != %q", id1, id2)
	}

	if len(id1) != 12 {
		t.Errorf("deriveRepoID length = %d, want 12", len(id1))
	}

	// Verify all characters are hex.
	for _, c := range id1 {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("deriveRepoID contains non-hex character: %c", c)
		}
	}
}

func TestDeriveRepoID_DifferentURLs(t *testing.T) {
	id1 := deriveRepoID("github.com/user/repo-a")
	id2 := deriveRepoID("github.com/user/repo-b")

	if id1 == id2 {
		t.Errorf("different URLs should produce different repo-ids: %q == %q", id1, id2)
	}
}

func TestDetectRepo(t *testing.T) {
	workDir := setupBareRemote(t)

	info, err := DetectRepo(workDir)
	if err != nil {
		t.Fatalf("DetectRepo failed: %v", err)
	}

	// Resolve symlinks for comparison (macOS /var -> /private/var).
	wantRoot, _ := filepath.EvalSymlinks(workDir)
	gotRoot, _ := filepath.EvalSymlinks(info.Root)
	if gotRoot != wantRoot {
		t.Errorf("Root = %q, want %q", info.Root, workDir)
	}

	if info.RemoteURL == "" {
		t.Error("RemoteURL should not be empty")
	}

	if info.NormalizedURL == "" {
		t.Error("NormalizedURL should not be empty")
	}

	if len(info.RepoID) != 12 {
		t.Errorf("RepoID length = %d, want 12", len(info.RepoID))
	}
}

func TestDetectRepo_NotARepo(t *testing.T) {
	dir := t.TempDir()

	_, err := DetectRepo(dir)
	if err == nil {
		t.Error("DetectRepo should fail for non-repo directory")
	}
}
