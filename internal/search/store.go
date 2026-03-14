package search

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const SchemaVersion = "1"

// Store wraps a SQLite database with dual FTS5 indexes.
type Store struct {
	db          *sql.DB
	projectRoot string
}

// OpenStore opens (or creates) the search index database at dbPath.
// projectRoot is recorded in index_meta for diagnostics and prune.
func OpenStore(dbPath, projectRoot string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	s := &Store{db: db, projectRoot: projectRoot}

	if err := s.ensureSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB exposes the underlying *sql.DB for use by indexer and searcher.
func (s *Store) DB() *sql.DB {
	return s.db
}

// ProjectRoot returns the project root stored in this store.
func (s *Store) ProjectRoot() string {
	return s.projectRoot
}

// ForceRebuild drops all tables and recreates the schema.
func (s *Store) ForceRebuild() error {
	tables := []string{"fts_trigram", "fts", "files", "index_meta"}
	for _, t := range tables {
		if _, err := s.db.Exec("DROP TABLE IF EXISTS " + t); err != nil {
			return fmt.Errorf("drop %s: %w", t, err)
		}
	}
	return s.createSchema()
}

func (s *Store) ensureSchema() error {
	// Check if index_meta exists to determine if this is a fresh db.
	var count int
	err := s.db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='index_meta'").Scan(&count)
	if err != nil {
		return fmt.Errorf("check schema: %w", err)
	}

	if count == 0 {
		return s.createSchema()
	}

	return s.checkVersion()
}

func (s *Store) checkVersion() error {
	var version string
	err := s.db.QueryRow("SELECT value FROM index_meta WHERE key='schema_version'").Scan(&version)
	if err != nil {
		// Missing or corrupt — recreate.
		return s.ForceRebuild()
	}

	if version == SchemaVersion {
		return nil
	}

	// Older version: migrate (for now, rebuild).
	if version < SchemaVersion {
		return s.ForceRebuild()
	}

	// Newer version: refuse.
	return fmt.Errorf("index schema version %s is newer than supported %s; run 'draft index --force' to rebuild", version, SchemaVersion)
}

func (s *Store) createSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS index_meta (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS files (
			id      INTEGER PRIMARY KEY,
			path    TEXT UNIQUE NOT NULL,
			hash    TEXT NOT NULL,
			mtime   INTEGER NOT NULL,
			indexed INTEGER NOT NULL
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS fts USING fts5(
			path,
			content,
			content_rowid,
			tokenize = 'porter unicode61'
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS fts_trigram USING fts5(
			path,
			content,
			content = '',
			content_rowid = id,
			tokenize = 'trigram'
		)`,
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("exec schema DDL: %w", err)
		}
	}

	// Populate index_meta.
	now := time.Now().UTC().Format(time.RFC3339)
	meta := map[string]string{
		"project_root":   s.projectRoot,
		"created_at":     now,
		"schema_version": SchemaVersion,
	}
	for k, v := range meta {
		if _, err := tx.Exec("INSERT OR REPLACE INTO index_meta (key, value) VALUES (?, ?)", k, v); err != nil {
			return fmt.Errorf("insert meta %s: %w", k, err)
		}
	}

	return tx.Commit()
}

// --- Files table CRUD ---

// FileRow represents a row in the files table.
type FileRow struct {
	ID      int64
	Path    string
	Hash    string
	Mtime   int64
	Indexed int64
}

// GetFile returns the file row for the given path, or nil if not found.
func (s *Store) GetFile(path string) (*FileRow, error) {
	row := s.db.QueryRow("SELECT id, path, hash, mtime, indexed FROM files WHERE path = ?", path)
	f := &FileRow{}
	err := row.Scan(&f.ID, &f.Path, &f.Hash, &f.Mtime, &f.Indexed)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return f, nil
}

// UpsertFile inserts or updates a file row and returns its ID.
func (s *Store) UpsertFile(tx *sql.Tx, path, hash string, mtime int64) (int64, error) {
	now := time.Now().Unix()
	res, err := tx.Exec(
		`INSERT INTO files (path, hash, mtime, indexed) VALUES (?, ?, ?, ?)
		 ON CONFLICT(path) DO UPDATE SET hash=excluded.hash, mtime=excluded.mtime, indexed=excluded.indexed`,
		path, hash, mtime, now,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateMtime updates only the mtime for a file (content unchanged).
func (s *Store) UpdateMtime(tx *sql.Tx, id, mtime int64) error {
	_, err := tx.Exec("UPDATE files SET mtime = ? WHERE id = ?", mtime, id)
	return err
}

// AllFilePaths returns all indexed file paths.
func (s *Store) AllFilePaths() (map[string]int64, error) {
	rows, err := s.db.Query("SELECT path, id FROM files")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	paths := make(map[string]int64)
	for rows.Next() {
		var path string
		var id int64
		if err := rows.Scan(&path, &id); err != nil {
			return nil, err
		}
		paths[path] = id
	}
	return paths, rows.Err()
}

// DeleteFile removes a file and its FTS entries.
// For contentless fts_trigram, we must supply the original content for deletion.
func (s *Store) DeleteFile(tx *sql.Tx, id int64) error {
	// Get the original content from the fts table before deleting.
	var path, content string
	err := tx.QueryRow("SELECT path, content FROM fts WHERE rowid = ?", id).Scan(&path, &content)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("read fts for delete: %w", err)
	}

	if _, err := tx.Exec("DELETE FROM fts WHERE rowid = ?", id); err != nil {
		return err
	}
	// Contentless FTS5 table requires special delete command with original values.
	if path != "" {
		if _, err := tx.Exec(
			"INSERT INTO fts_trigram(fts_trigram, rowid, path, content) VALUES('delete', ?, ?, ?)",
			id, path, content,
		); err != nil {
			return err
		}
	}
	if _, err := tx.Exec("DELETE FROM files WHERE id = ?", id); err != nil {
		return err
	}
	return nil
}

// --- FTS operations ---

// InsertFTS inserts content into both FTS5 tables for the given file ID.
func (s *Store) InsertFTS(tx *sql.Tx, id int64, path, content string) error {
	if _, err := tx.Exec("INSERT INTO fts (rowid, path, content) VALUES (?, ?, ?)", id, path, content); err != nil {
		return fmt.Errorf("insert fts: %w", err)
	}
	if _, err := tx.Exec("INSERT INTO fts_trigram (rowid, path, content) VALUES (?, ?, ?)", id, path, content); err != nil {
		return fmt.Errorf("insert fts_trigram: %w", err)
	}
	return nil
}

// DeleteFTS removes FTS entries for the given file ID from both tables.
// Reads original content from fts to supply to contentless fts_trigram delete.
func (s *Store) DeleteFTS(tx *sql.Tx, id int64) error {
	var path, content string
	err := tx.QueryRow("SELECT path, content FROM fts WHERE rowid = ?", id).Scan(&path, &content)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("read fts for delete: %w", err)
	}

	if _, err := tx.Exec("DELETE FROM fts WHERE rowid = ?", id); err != nil {
		return err
	}
	if path != "" {
		if _, err := tx.Exec(
			"INSERT INTO fts_trigram(fts_trigram, rowid, path, content) VALUES('delete', ?, ?, ?)",
			id, path, content,
		); err != nil {
			return err
		}
	}
	return nil
}

// --- Meta queries ---

// FileCount returns the number of indexed files.
func (s *Store) FileCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT count(*) FROM files").Scan(&count)
	return count, err
}

// Meta returns a value from index_meta.
func (s *Store) Meta(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM index_meta WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// Optimize runs PRAGMA optimize on the database.
func (s *Store) Optimize() error {
	_, err := s.db.Exec("PRAGMA optimize")
	return err
}

// Begin starts a new transaction.
func (s *Store) Begin() (*sql.Tx, error) {
	return s.db.Begin()
}
