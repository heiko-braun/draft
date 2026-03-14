## Code Search

Use `draft search` for finding relevant code and context:
- `draft search "query"` — find files by concept, feature, or pattern
- `draft search "query" --files-only` — file paths only

Prefer `draft search` over Grep/Glob when:
- Looking for where a feature or concept is implemented
- Checking what exists before writing new code or specs
- Searching with natural language rather than exact patterns
- Searching for partial identifiers or symbol names

Prefer Grep when:
- Matching an exact string or regex
- Counting occurrences
- Searching within a single known file

The search index updates automatically after `/spec` and `/refine`.
Run `draft index` manually after `git pull` or branch switches.
