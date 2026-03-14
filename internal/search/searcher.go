package search

import (
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	weightFTS     = 0.6
	weightTrigram = 0.4
	defaultLimit  = 20
)

// QueryType classifies a search query.
type QueryType int

const (
	QueryNaturalLanguage QueryType = iota
	QuerySubstring
	QueryMixed
)

// SearchResult represents a single search hit.
type SearchResult struct {
	Path    string
	Score   float64
	Snippet string
}

// Search runs a query against the index and returns ranked results.
func Search(store *Store, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = defaultLimit
	}

	qt := ClassifyQuery(query)

	var ftsResults, triResults []SearchResult
	var err error

	switch qt {
	case QueryNaturalLanguage:
		ftsResults, err = searchFTS(store, query, limit)
		if err != nil {
			return nil, err
		}
		return ftsResults, nil

	case QuerySubstring:
		if len(query) < 3 {
			// Trigram requires at least 3 chars.
			return nil, nil
		}
		triResults, err = searchTrigram(store, query, limit)
		if err != nil {
			return nil, err
		}
		return triResults, nil

	case QueryMixed:
		ftsResults, err = searchFTS(store, query, limit)
		if err != nil {
			return nil, err
		}
		if len(query) >= 3 {
			triResults, err = searchTrigram(store, query, limit)
			if err != nil {
				return nil, err
			}
		}
		return mergeResults(ftsResults, triResults, limit), nil
	}

	return nil, nil
}

// ClassifyQuery determines how to route a query.
func ClassifyQuery(query string) QueryType {
	hasSpaces := strings.Contains(query, " ")

	// camelCase or PascalCase: uppercase letter mid-word
	camelCase := regexp.MustCompile(`[a-z][A-Z]`)
	isCamel := camelCase.MatchString(query)

	// snake_case
	isSnake := strings.Contains(query, "_") && !hasSpaces

	if hasSpaces && !isCamel && !isSnake {
		return QueryNaturalLanguage
	}
	if !hasSpaces && (isCamel || isSnake) {
		return QuerySubstring
	}
	if hasSpaces {
		return QueryMixed
	}
	// Single word without special patterns — could be either.
	// Route to both to maximize recall.
	return QueryMixed
}

func searchFTS(store *Store, query string, limit int) ([]SearchResult, error) {
	// Escape special FTS5 characters in query.
	escaped := escapeFTS5(query)

	rows, err := store.DB().Query(`
		SELECT f.path,
		       snippet(fts, 1, '»', '«', '…', 32) as snippet,
		       bm25(fts, 5.0, 1.0) as score
		FROM fts
		JOIN files f ON f.id = fts.rowid
		WHERE fts MATCH ?
		ORDER BY score
		LIMIT ?
	`, escaped, limit)
	if err != nil {
		return nil, fmt.Errorf("fts query: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.Path, &r.Snippet, &r.Score); err != nil {
			return nil, err
		}
		// BM25 returns negative scores (lower = better). Negate for consistent sorting.
		r.Score = -r.Score
		results = append(results, r)
	}
	return results, rows.Err()
}

func searchTrigram(store *Store, query string, limit int) ([]SearchResult, error) {
	rows, err := store.DB().Query(`
		SELECT f.path,
		       bm25(fts_trigram, 5.0, 1.0) as score
		FROM fts_trigram
		JOIN files f ON f.id = fts_trigram.rowid
		WHERE fts_trigram MATCH ?
		ORDER BY score
		LIMIT ?
	`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("trigram query: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.Path, &r.Score); err != nil {
			return nil, err
		}
		r.Score = -r.Score
		results = append(results, r)
	}
	return results, rows.Err()
}

func mergeResults(fts, tri []SearchResult, limit int) []SearchResult {
	normScores(fts)
	normScores(tri)

	merged := make(map[string]*SearchResult)

	for i := range fts {
		r := fts[i]
		merged[r.Path] = &SearchResult{
			Path:    r.Path,
			Score:   weightFTS * r.Score,
			Snippet: r.Snippet,
		}
	}

	for i := range tri {
		r := tri[i]
		if existing, ok := merged[r.Path]; ok {
			existing.Score += weightTrigram * r.Score
		} else {
			merged[r.Path] = &SearchResult{
				Path:  r.Path,
				Score: weightTrigram * r.Score,
			}
		}
	}

	results := make([]SearchResult, 0, len(merged))
	for _, r := range merged {
		results = append(results, *r)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

func normScores(results []SearchResult) {
	if len(results) == 0 {
		return
	}

	min, max := math.Inf(1), math.Inf(-1)
	for _, r := range results {
		if r.Score < min {
			min = r.Score
		}
		if r.Score > max {
			max = r.Score
		}
	}

	spread := max - min
	if spread == 0 {
		for i := range results {
			results[i].Score = 1.0
		}
		return
	}

	for i := range results {
		results[i].Score = (results[i].Score - min) / spread
	}
}

// escapeFTS5 wraps each token in double quotes to avoid FTS5 syntax errors
// from special characters like -, *, etc.
func escapeFTS5(query string) string {
	tokens := strings.Fields(query)
	for i, t := range tokens {
		// Wrap in double quotes, escaping any internal double quotes.
		t = strings.ReplaceAll(t, `"`, `""`)
		tokens[i] = `"` + t + `"`
	}
	return strings.Join(tokens, " ")
}

// FormatResults formats search results as markdown with fenced code blocks.
func FormatResults(results []SearchResult) string {
	if len(results) == 0 {
		return "No results found.\n"
	}

	var sb strings.Builder
	for _, r := range results {
		fmt.Fprintf(&sb, "%s (score: %.2f)\n", r.Path, r.Score)
		if r.Snippet != "" {
			lang := langFromExt(r.Path)
			fmt.Fprintf(&sb, "```%s\n%s\n```\n", lang, r.Snippet)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// langFromExt returns a markdown language tag for the given file path.
// Returns empty string for unknown extensions.
func langFromExt(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	lang, ok := extLangs[ext]
	if !ok {
		return ""
	}
	return lang
}

var extLangs = map[string]string{
	".go":    "go",
	".js":    "javascript",
	".ts":    "typescript",
	".jsx":   "jsx",
	".tsx":   "tsx",
	".py":    "python",
	".rb":    "ruby",
	".rs":    "rust",
	".java":  "java",
	".kt":    "kotlin",
	".c":     "c",
	".cpp":   "cpp",
	".h":     "c",
	".hpp":   "cpp",
	".cs":    "csharp",
	".swift": "swift",
	".sh":    "bash",
	".bash":  "bash",
	".zsh":   "bash",
	".md":    "markdown",
	".yaml":  "yaml",
	".yml":   "yaml",
	".json":  "json",
	".toml":  "toml",
	".xml":   "xml",
	".html":  "html",
	".css":   "css",
	".scss":  "scss",
	".sql":   "sql",
	".proto": "protobuf",
	".tf":    "hcl",
	".lua":   "lua",
	".ex":    "elixir",
	".exs":   "elixir",
	".erl":   "erlang",
	".hs":    "haskell",
	".ml":    "ocaml",
	".r":     "r",
	".php":   "php",
	".pl":    "perl",
	".vim":   "vim",
	".el":    "lisp",
	".clj":   "clojure",
	".scala": "scala",
	".dart":  "dart",
	".zig":   "zig",
}

// StatusInfo holds information for the --status flag.
type StatusInfo struct {
	DBPath      string
	ProjectRoot string
	FileCount   int
	LastIndexed string
	SizeBytes   int64
}
