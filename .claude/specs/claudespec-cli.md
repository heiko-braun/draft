# Feature: claudespec CLI Tool

## Goal

Create a Go-based CLI tool that bootstraps the Claude spec-driven development workflow into any Git repository (greenfield or brownfield) by copying the `.claude/` directory structure and files from this repository.

## Acceptance Criteria

- [x] CLI binary named `claudespec` can be built for macOS (amd64/arm64)
- [x] Users can install via `go install` or download pre-built binaries from GitHub releases
- [x] Running `claudespec init` in a target directory creates `.claude/commands/` and `.claude/specs/` directories
- [x] All files are copied: `spec.md`, `implement.md`, `plan.md` commands and `TEMPLATE.md` spec template
- [x] When files already exist, CLI warns and stops without making changes
- [x] `--force` flag overwrites existing files and logs what was overwritten
- [x] CLI displays summary output showing count of files created/skipped
- [x] `--version` flag displays the CLI version
- [x] Returns appropriate exit codes (0 for success, non-zero for errors/conflicts)

## Approach

Build a Go CLI using the `cobra` library for command structure. The `init` command will:

1. Embed the `.claude/` directory contents into the Go binary using `embed` package
2. Check if target directory is writable
3. Scan for existing files in `.claude/commands/` and `.claude/specs/`
4. If conflicts found (files exist) and no `--force` flag, display warning with list of conflicts and exit with code 1
5. If `--force` or no conflicts, create directories and copy files
6. Display summary: "Successfully created X files in .claude/"

Use GitHub Actions to build binaries for macOS (amd64/arm64) and publish to GitHub releases. Version will be injected at build time via `-ldflags`.

## Out of Scope

- Linux and Windows binaries (macOS only for initial release)
- Dry-run mode (`--dry-run`)
- Interactive conflict resolution
- Update command to sync with latest template changes
- Validation that target is a Git repository
- Customization/templating of copied files
- Verbose output mode

## Notes

The CLI will be located in this repository under `cmd/claudespec/`. The embedded files will be sourced from `.claude/` directory at build time. Consider adding a simple README in the root explaining installation and usage.
