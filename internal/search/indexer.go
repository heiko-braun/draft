package search

import (
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
	"github.com/zeebo/xxh3"
)

const (
	maxFileSize     = 1 << 20 // 1 MB
	binaryCheckSize = 8192
)

// hardcoded directories to always skip
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	".hg":          true,
	".svn":         true,
	"__pycache__":  true,
	".idea":        true,
	".vscode":      true,
}

// IndexResult reports what happened during an index run.
type IndexResult struct {
	FilesIndexed   int
	FilesSkipped   int
	FilesDeleted   int
	FilesUnchanged int
}

// Index walks the project tree and updates both FTS5 indexes incrementally.
// If force is true, the store is rebuilt from scratch first.
func Index(store *Store, projectRoot string, force bool) (*IndexResult, error) {
	if force {
		if err := store.ForceRebuild(); err != nil {
			return nil, fmt.Errorf("force rebuild: %w", err)
		}
	}

	gi := loadGitignore(projectRoot)

	// Resolve the index db path to skip it during walk.
	dbPath := ""
	if dbPathVal, err := IndexPath(projectRoot, ""); err == nil {
		dbPath = dbPathVal
	}

	// Get all currently indexed paths so we can detect deletions.
	existing, err := store.AllFilePaths()
	if err != nil {
		return nil, fmt.Errorf("load existing paths: %w", err)
	}
	seen := make(map[string]bool)

	result := &IndexResult{}

	tx, err := store.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	err = filepath.WalkDir(projectRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}

		// Relative path for matching and storage.
		rel, err := filepath.Rel(projectRoot, path)
		if err != nil {
			return nil
		}

		if d.IsDir() {
			name := d.Name()
			if skipDirs[name] {
				return filepath.SkipDir
			}
			if gi != nil && gi.MatchesPath(rel+"/") {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip the index database itself.
		if dbPath != "" && path == dbPath {
			return nil
		}

		// Skip gitignored files.
		if gi != nil && gi.MatchesPath(rel) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		// Skip large files.
		if info.Size() > maxFileSize {
			result.FilesSkipped++
			return nil
		}

		seen[rel] = true

		mtime := info.ModTime().Unix()

		// Check existing record.
		existing, err := store.GetFile(rel)
		if err != nil {
			return fmt.Errorf("get file %s: %w", rel, err)
		}

		// mtime fast path: unchanged → skip entirely.
		if existing != nil && existing.Mtime == mtime {
			result.FilesUnchanged++
			return nil
		}

		// Read file content.
		content, err := os.ReadFile(path)
		if err != nil {
			return nil // skip unreadable
		}

		// Binary check.
		if isBinary(content) {
			result.FilesSkipped++
			return nil
		}

		// Compute hash.
		h := xxh3.Hash128(content)
		hb := h.Bytes()
		hashStr := hex.EncodeToString(hb[:])

		// If hash unchanged (mtime-only change), just update mtime.
		if existing != nil && existing.Hash == hashStr {
			if err := store.UpdateMtime(tx, existing.ID, mtime); err != nil {
				return fmt.Errorf("update mtime %s: %w", rel, err)
			}
			result.FilesUnchanged++
			return nil
		}

		// Content changed (or new file): upsert file + FTS.
		if existing != nil {
			if err := store.DeleteFTS(tx, existing.ID); err != nil {
				return fmt.Errorf("delete fts %s: %w", rel, err)
			}
		}

		id, err := store.UpsertFile(tx, rel, hashStr, mtime)
		if err != nil {
			return fmt.Errorf("upsert file %s: %w", rel, err)
		}

		// For upsert (ON CONFLICT), LastInsertId may not return the existing id.
		// Re-fetch if needed.
		if existing != nil {
			id = existing.ID
		}

		if err := store.InsertFTS(tx, id, rel, string(content)); err != nil {
			return fmt.Errorf("insert fts %s: %w", rel, err)
		}

		result.FilesIndexed++
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk: %w", err)
	}

	// Delete files that no longer exist on disk.
	for path, id := range existing {
		if !seen[path] {
			if err := store.DeleteFile(tx, id); err != nil {
				return nil, fmt.Errorf("delete %s: %w", path, err)
			}
			result.FilesDeleted++
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	if err := store.Optimize(); err != nil {
		return nil, fmt.Errorf("optimize: %w", err)
	}

	return result, nil
}

// loadGitignore loads .gitignore from the project root, if present.
func loadGitignore(root string) *ignore.GitIgnore {
	path := filepath.Join(root, ".gitignore")
	gi, err := ignore.CompileIgnoreFile(path)
	if err != nil {
		return nil
	}
	return gi
}

// isBinary checks the first binaryCheckSize bytes for null bytes.
func isBinary(content []byte) bool {
	check := content
	if len(check) > binaryCheckSize {
		check = check[:binaryCheckSize]
	}
	for _, b := range check {
		if b == 0 {
			return true
		}
	}
	return false
}

// PruneIndexes scans the cache directory and deletes indexes whose
// project_root no longer exists on disk.
func PruneIndexes() ([]string, error) {
	cacheDir, err := cacheBaseDir()
	if err != nil {
		return nil, err
	}
	cacheDir = filepath.Join(cacheDir) // already includes "draft"

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var pruned []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".db") {
			continue
		}

		dbPath := filepath.Join(cacheDir, e.Name())
		root, err := readProjectRoot(dbPath)
		if err != nil {
			continue // skip corrupt files
		}

		if _, err := os.Stat(root); os.IsNotExist(err) {
			os.Remove(dbPath)
			// Also remove WAL and SHM files.
			os.Remove(dbPath + "-wal")
			os.Remove(dbPath + "-shm")
			pruned = append(pruned, root)
		}
	}

	return pruned, nil
}

// IndexInfo holds summary info about one index database.
type IndexInfo struct {
	ProjectRoot string
	FileCount   int
	SizeBytes   int64
	LastIndexed string
	DBPath      string
}

// ListIndexes returns info about all index databases in the cache directory.
func ListIndexes() ([]IndexInfo, error) {
	cacheDir, err := cacheBaseDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var infos []IndexInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".db") {
			continue
		}

		dbPath := filepath.Join(cacheDir, e.Name())
		info, err := readIndexInfo(dbPath)
		if err != nil {
			continue
		}
		infos = append(infos, info)
	}

	return infos, nil
}

func readProjectRoot(dbPath string) (string, error) {
	info, err := readIndexInfo(dbPath)
	if err != nil {
		return "", err
	}
	return info.ProjectRoot, nil
}

func readIndexInfo(dbPath string) (IndexInfo, error) {
	s, err := OpenStore(dbPath, "")
	if err != nil {
		return IndexInfo{}, err
	}
	defer s.Close()

	root, err := s.Meta("project_root")
	if err != nil {
		return IndexInfo{}, err
	}

	count, err := s.FileCount()
	if err != nil {
		return IndexInfo{}, err
	}

	lastIndexed, _ := s.Meta("created_at")

	fi, err := os.Stat(dbPath)
	if err != nil {
		return IndexInfo{}, err
	}

	return IndexInfo{
		ProjectRoot: root,
		FileCount:   count,
		SizeBytes:   fi.Size(),
		LastIndexed: lastIndexed,
		DBPath:      dbPath,
	}, nil
}
