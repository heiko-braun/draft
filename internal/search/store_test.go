package search

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := OpenStore(dbPath, "/test/project")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenStore_CreatesSchema(t *testing.T) {
	s := tempStore(t)

	// Verify all four tables exist.
	tables := []string{"index_meta", "files", "fts", "fts_trigram"}
	for _, name := range tables {
		var count int
		err := s.DB().QueryRow(
			"SELECT count(*) FROM sqlite_master WHERE name = ?", name,
		).Scan(&count)
		if err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			t.Errorf("table %q not found", name)
		}
	}
}

func TestOpenStore_MetaPopulated(t *testing.T) {
	s := tempStore(t)

	root, err := s.Meta("project_root")
	if err != nil || root != "/test/project" {
		t.Errorf("project_root = %q, err = %v", root, err)
	}

	ver, err := s.Meta("schema_version")
	if err != nil || ver != SchemaVersion {
		t.Errorf("schema_version = %q, err = %v", ver, err)
	}

	created, err := s.Meta("created_at")
	if err != nil || created == "" {
		t.Errorf("created_at = %q, err = %v", created, err)
	}
}

func TestOpenStore_VersionMatch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	// First open creates.
	s1, err := OpenStore(dbPath, "/test/project")
	if err != nil {
		t.Fatal(err)
	}
	s1.Close()

	// Second open with same version succeeds.
	s2, err := OpenStore(dbPath, "/test/project")
	if err != nil {
		t.Fatal(err)
	}
	s2.Close()
}

func TestOpenStore_NewerVersionFails(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	s, err := OpenStore(dbPath, "/test/project")
	if err != nil {
		t.Fatal(err)
	}
	// Set version to something newer.
	_, err = s.DB().Exec("UPDATE index_meta SET value = '999' WHERE key = 'schema_version'")
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	// Re-open should fail.
	_, err = OpenStore(dbPath, "/test/project")
	if err == nil {
		t.Fatal("expected error for newer schema version")
	}
}

func TestOpenStore_OlderVersionRebuilds(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	s, err := OpenStore(dbPath, "/test/project")
	if err != nil {
		t.Fatal(err)
	}
	// Insert a file to verify rebuild clears data.
	tx, _ := s.Begin()
	s.UpsertFile(tx, "old.go", "abc", 100)
	tx.Commit()
	// Set version to something older.
	_, err = s.DB().Exec("UPDATE index_meta SET value = '0' WHERE key = 'schema_version'")
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	// Re-open should rebuild (older version).
	s2, err := OpenStore(dbPath, "/test/project")
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	count, err := s2.FileCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0 files after rebuild, got %d", count)
	}
}

func TestOpenStore_CacheDirCreated(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "deep", "nested", "cache")
	dbPath := filepath.Join(dir, "test.db")

	s, err := OpenStore(dbPath, "/test/project")
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("cache directory was not created")
	}
}

func TestStore_FilesCRUD(t *testing.T) {
	s := tempStore(t)

	// Insert a file.
	tx, _ := s.Begin()
	id, err := s.UpsertFile(tx, "main.go", "hash1", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.InsertFTS(tx, id, "main.go", "package main"); err != nil {
		t.Fatal(err)
	}
	tx.Commit()

	// Get it back.
	f, err := s.GetFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if f == nil {
		t.Fatal("expected file, got nil")
	}
	if f.Hash != "hash1" || f.Mtime != 1000 {
		t.Errorf("got hash=%q mtime=%d", f.Hash, f.Mtime)
	}

	// AllFilePaths.
	paths, err := s.AllFilePaths()
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Errorf("expected 1 path, got %d", len(paths))
	}

	// FileCount.
	count, err := s.FileCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}

	// Delete.
	tx, _ = s.Begin()
	if err := s.DeleteFile(tx, f.ID); err != nil {
		t.Fatal(err)
	}
	tx.Commit()

	f2, err := s.GetFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if f2 != nil {
		t.Error("expected nil after delete")
	}
}

func TestStore_UpdateMtime(t *testing.T) {
	s := tempStore(t)

	tx, _ := s.Begin()
	id, _ := s.UpsertFile(tx, "a.go", "h", 100)
	tx.Commit()

	tx, _ = s.Begin()
	if err := s.UpdateMtime(tx, id, 200); err != nil {
		t.Fatal(err)
	}
	tx.Commit()

	f, _ := s.GetFile("a.go")
	if f.Mtime != 200 {
		t.Errorf("mtime = %d, want 200", f.Mtime)
	}
}

func TestStore_ForceRebuild(t *testing.T) {
	s := tempStore(t)

	// Add data.
	tx, _ := s.Begin()
	id, _ := s.UpsertFile(tx, "x.go", "h", 1)
	s.InsertFTS(tx, id, "x.go", "content")
	tx.Commit()

	// Rebuild.
	if err := s.ForceRebuild(); err != nil {
		t.Fatal(err)
	}

	count, _ := s.FileCount()
	if count != 0 {
		t.Errorf("expected 0 after rebuild, got %d", count)
	}

	// Verify FTS tables are empty.
	var ftsCount int
	s.DB().QueryRow("SELECT count(*) FROM fts").Scan(&ftsCount)
	if ftsCount != 0 {
		t.Errorf("fts not empty after rebuild")
	}
}

func TestStore_FTSContentlessTrigramNoContent(t *testing.T) {
	s := tempStore(t)

	tx, _ := s.Begin()
	id, _ := s.UpsertFile(tx, "test.go", "h", 1)
	s.InsertFTS(tx, id, "test.go", "some content here for testing")
	tx.Commit()

	// fts_trigram is contentless — reading content should return empty.
	var content sql.NullString
	err := s.DB().QueryRow("SELECT content FROM fts_trigram WHERE rowid = ?", id).Scan(&content)
	if err != nil {
		t.Fatal(err)
	}
	if content.Valid && content.String != "" {
		t.Errorf("contentless table returned content: %q", content.String)
	}
}

func TestStore_Optimize(t *testing.T) {
	s := tempStore(t)
	if err := s.Optimize(); err != nil {
		t.Fatal(err)
	}
}
