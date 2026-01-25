# Feature: Remote Template Loading from GitHub

## Goal

Decouple the CLI from hardcoded template files by fetching `.claude/` templates from GitHub releases at runtime, allowing users to always get the latest templates without recompiling the CLI while supporting local overrides for testing and customization via environment variable.

## Acceptance Criteria

- [x] By default, `draft init` fetches templates from the latest GitHub release of `heiko-braun/draft`
- [x] Users can specify a version with `--version v1.2.0` to fetch templates from a specific release tag
- [x] Users can set `DRAFT_TEMPLATES=/local/path` environment variable to use local files instead of fetching from GitHub
- [x] When environment variable is set, CLI validates the directory contains `.claude/commands/` and `.claude/specs/` subdirectories
- [x] If validation fails, CLI exits with clear error message indicating what's missing
- [x] When environment variable is set and validated, GitHub fetching is completely skipped
- [x] When GitHub is unreachable or request fails, CLI falls back to embedded templates
- [x] Embedded templates are kept minimal (same files as currently in `cmd/draft/templates/.claude/`)
- [x] CLI displays clear messaging about template source (GitHub release/version, local path, or fallback)
- [x] GitHub requests have reasonable timeout (e.g., 10 seconds)
- [x] No caching of downloaded templates between runs

## Approach

Create a template loader abstraction in `internal/templates/` with three strategies:
1. **GitHubLoader**: Fetches from GitHub releases API, downloads archive, extracts `.claude/` directory
2. **LocalLoader**: Reads from filesystem path from `DRAFT_TEMPLATES` environment variable
3. **EmbeddedLoader**: Falls back to `embed.FS` templates bundled in binary

The `init` command will:
1. Check if `DRAFT_TEMPLATES` env var is set → use LocalLoader
2. Otherwise, attempt GitHubLoader (with `--version` if specified, else latest release)
3. On GitHub failure (network error, timeout, 404), fall back to EmbeddedLoader
4. Pass resolved templates to existing init logic

Use GitHub's releases API (`GET /repos/h010198/claude-spec-driven/releases/latest` or `/releases/tags/{version}`) to get the tarball URL, download and extract `.claude/` contents in-memory.

## Out of Scope

- Caching downloaded templates between runs
- `--templates` flag for local path (use environment variable instead)
- Conventional template location (e.g., `~/.draft/templates/`)
- Merging local and remote templates (local completely replaces remote)
- Interactive template selection or customization during init
- Template validation or schema checking
- Support for loading from non-GitHub sources
- Branch-based template loading (releases/tags only)

## Notes

The embedded templates serve as a safety fallback and ensure the CLI works offline. They should be synced with the repository's `.claude/` directory before each release.

The `DRAFT_TEMPLATES` environment variable should point to a directory containing a `.claude/` subdirectory with the standard structure:
```
$DRAFT_TEMPLATES/
  .claude/
    commands/
      spec.md
      implement.md
      refine.md
    specs/
      TEMPLATE.md
```

Consider adding a `--dry-run` flag in the future to preview what would be installed without making changes.
