package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadCacheFresh(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "latest.json")
	data, _ := json.Marshal(noticeCache{Tag: "v1.2.3", CheckedAt: time.Now()})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	tag, fresh := readCache(path)
	if tag != "v1.2.3" || !fresh {
		t.Errorf("got (%q, %v), want (v1.2.3, true)", tag, fresh)
	}
}

func TestReadCacheStale(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "latest.json")
	data, _ := json.Marshal(noticeCache{Tag: "v1.2.3", CheckedAt: time.Now().Add(-25 * time.Hour)})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	_, fresh := readCache(path)
	if fresh {
		t.Error("expected stale cache, got fresh")
	}
}

func TestReadCacheMissing(t *testing.T) {
	_, fresh := readCache(filepath.Join(t.TempDir(), "nope.json"))
	if fresh {
		t.Error("missing cache should not be fresh")
	}
}

func TestWriteCacheRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "latest.json")
	if err := writeCache(path, "v9.9.9"); err != nil {
		t.Fatal(err)
	}
	tag, fresh := readCache(path)
	if tag != "v9.9.9" || !fresh {
		t.Errorf("got (%q, %v), want (v9.9.9, true)", tag, fresh)
	}
}

func TestShouldCheckSkipsDevAndNonTTY(t *testing.T) {
	// dev version → skip
	if shouldCheck("dev", os.Stderr) {
		t.Error("dev version should skip check")
	}
	// non-*os.File writer → skip
	if shouldCheck("v1.0.0", &writerStub{}) {
		t.Error("non-file writer should skip check")
	}
	// env var override → skip
	t.Setenv("DRAFT_NO_UPDATE_CHECK", "1")
	if shouldCheck("v1.0.0", os.Stderr) {
		t.Error("DRAFT_NO_UPDATE_CHECK should skip check")
	}
}

type writerStub struct{}

func (w *writerStub) Write(p []byte) (int, error) { return len(p), nil }
