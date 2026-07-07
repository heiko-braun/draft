package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/term"
)

const (
	noticeCacheTTL  = 24 * time.Hour
	noticeCacheFile = "latest.json"
)

type noticeCache struct {
	Tag       string    `json:"tag"`
	CheckedAt time.Time `json:"checked_at"`
}

// maybePrintUpdateNotice checks (with 24h cache) whether a newer release exists
// and, if so, prints a one-line notice to w. All errors are swallowed — the
// notice is a nice-to-have and must never interfere with normal command output.
func maybePrintUpdateNotice(w io.Writer, currentVersion string) {
	if !shouldCheck(currentVersion, w) {
		return
	}

	cachePath, err := cacheFilePath()
	if err != nil {
		return
	}

	latest, fresh := readCache(cachePath)
	if !fresh {
		fetched, err := fetchLatestVersion()
		if err != nil {
			return
		}
		latest = fetched
		_ = writeCache(cachePath, latest)
	}

	if latest == "" {
		return
	}
	if !parseSemver(latest).greaterThan(parseSemver(currentVersion)) {
		return
	}

	fmt.Fprintf(w, "\nUpdate available: %s → %s. Run 'draft update' to install.\n", currentVersion, latest)
}

func shouldCheck(currentVersion string, w io.Writer) bool {
	if currentVersion == "" || currentVersion == "dev" {
		return false
	}
	if os.Getenv("DRAFT_NO_UPDATE_CHECK") != "" {
		return false
	}
	// Only when stderr is a TTY — we print the notice to stderr so it doesn't
	// pollute piped stdout. If stderr isn't a terminal, stay silent.
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

func cacheFilePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	draftDir := filepath.Join(dir, "draft")
	if err := os.MkdirAll(draftDir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(draftDir, noticeCacheFile), nil
}

func readCache(path string) (tag string, fresh bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var c noticeCache
	if err := json.Unmarshal(data, &c); err != nil {
		return "", false
	}
	if time.Since(c.CheckedAt) > noticeCacheTTL {
		return c.Tag, false
	}
	return c.Tag, true
}

func writeCache(path, tag string) error {
	data, err := json.Marshal(noticeCache{Tag: tag, CheckedAt: time.Now()})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
