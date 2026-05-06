# Draft Index Search Design

The search system has three layers: **Store** (SQLite), **Indexer** (populates the store), and **Searcher** (queries it).

---

## 1. Indexing (`draft index`)

**`internal/search/indexer.go`** — Walks the project tree and builds the index:

- **Incremental updates**: Uses **mtime** as a fast path — if unchanged, skip. If mtime changed but **xxh3 hash** is the same, just update mtime. Only re-indexes when content actually changed.
- **Skips**: `.git`, `node_modules`, `.gitignore`'d files, binary files (null byte check), files >1MB
- **Force rebuild**: `--force` drops all tables and re-creates from scratch
- **Prune**: Removes orphaned indexes for deleted projects

## 2. Storage (`internal/search/store.go`)

**SQLite with WAL mode** containing:

- **`files`** table — tracks `path`, `hash` (xxh3), `mtime`, `indexed` timestamp
- **`fts`** — FTS5 virtual table with **`porter unicode61`** tokenizer (stemming + natural language)
- **`fts_trigram`** — FTS5 virtual table with **`trigram`** tokenizer (substring/pattern matching)
- **`index_meta`** — stores `project_root`, `schema_version`, `created_at`

Both FTS tables index `path` and `content`. The trigram table is **contentless** (`content = ''`) to save space.

## 3. Search (`draft search <query>`)

**`internal/search/searcher.go`** — Routes queries through a classification system:

### Query Classification (`ClassifyQuery`)

| Pattern | Type | Strategy |
|---|---|---|
| Has spaces, no camelCase/snake_case | `NaturalLanguage` | FTS only |
| No spaces + camelCase or snake_case | `Substring` | Trigram only |
| Spaces + camelCase/snake_case, or single word | `Mixed` | Both, merged |

### Search Strategies

- **FTS search**: Uses `BM25` ranking with path weighted 5× over content. Returns snippets via `snippet()`. Special chars escaped by wrapping tokens in double quotes.
- **Trigram search**: Uses `BM25` ranking (same weights). Requires ≥3 char query. No snippets.
- **Mixed merge**: Normalizes scores to [0,1], then combines with **60% FTS + 40% trigram** weighting. Deduplicates by path.

### Output

Results are formatted as markdown with file paths, scores, and fenced code block snippets (with language detection from file extension).
