# Contributing to Draft

Thank you for your interest in contributing! This guide will help you set up your development environment and understand the project structure.

## Getting Started

### Prerequisites

- Go 1.24 or later
- Git
- Make

### Setting Up Your Development Environment

1. **Fork and clone the repository:**

```bash
git clone https://github.com/heiko-braun/draft.git
cd draft
```

2. **Install dependencies:**

```bash
go mod download
```

3. **Install git hooks (recommended):**

```bash
make install-hooks
```

This installs a pre-commit hook that automatically checks your code before each commit.

## Development Workflow

### Building the CLI

```bash
# Build the draft binary
make build

# The binary will be in bin/draft
./bin/draft --version
```

### Running Tests

```bash
make test
```

### Code Quality

Before committing, ensure your code passes these checks:

```bash
# Format your code
make fmt

# Run static analysis
make vet
```

## Git Hooks

We use git hooks to maintain code quality. When you run `make install-hooks`, a pre-commit hook is installed that automatically:

1. **Checks code formatting** - Ensures all Go code follows standard formatting
2. **Validates dependencies** - Runs `go mod tidy` and checks for changes
3. **Runs static analysis** - Executes `go vet` on changed packages only

### How the Pre-Commit Hook Works

The hook (`scripts/pre-commit`):
- Stashes unstaged changes before running checks
- Only checks packages that have changed (for efficiency)
- Restores your working directory state after running
- Prevents commits if any check fails

### Bypassing the Hook

If you need to commit without running the hook (not recommended):

```bash
git commit --no-verify
```

## Makefile Targets

The project includes several helpful Make targets:

| Target | Description |
|--------|-------------|
| `make build` | Build the draft binary to `bin/draft` |
| `make install` | Install draft to `$GOPATH/bin` |
| `make test` | Run all tests |
| `make fmt` | Format all Go code with `go fmt` |
| `make vet` | Run `go vet` static analysis |
| `make install-hooks` | Install git pre-commit hooks |
| `make clean` | Remove build artifacts |
| `make run ARGS="init"` | Run the CLI with arguments (for development) |

## Build Configuration

The build process injects version information at compile time using ldflags:

- **VERSION**: Git tag or "dev" (from `git describe`)
- **COMMIT**: Short commit hash
- **DATE**: Build timestamp (UTC)

These are accessible via the `--version` flag:

```bash
./bin/draft --version
# Output: draft version v1.0.0 (commit: abc123, built: 2024-01-25T10:00:00Z)
```

## Release Process

Releases are automated using [GoReleaser](https://goreleaser.com/) and GitHub Actions.

### Creating a Release

1. **Tag a new version:**

```bash
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

2. **GitHub Actions automatically:**
   - Builds binaries for macOS (amd64 and arm64)
   - Generates checksums
   - Creates a GitHub release with binaries attached
   - Generates release notes from commits

### Release Configuration

The release process is configured in:
- `.goreleaser.yaml` - GoReleaser configuration
- `.github/workflows/release.yml` - GitHub Actions workflow

## Continuous Integration

Pull requests are automatically verified by GitHub Actions (`.github/workflows/pr-verify.yml`):

- ✅ Code formatting check
- ✅ Static analysis (`go vet`)
- ✅ Build verification
- ✅ Test execution

All checks must pass before a PR can be merged.

## Project Structure

```
.
├── .claude/                    # Spec-driven development commands
│   ├── commands/              # Slash commands (plan, spec, implement, refine)
│   └── specs/                 # Feature specifications
├── .github/
│   └── workflows/             # CI/CD workflows
│       ├── pr-verify.yml     # PR verification checks
│       └── release.yml       # Release automation
├── cmd/
│   └── draft/                # CLI entry point
│       ├── main.go
│       └── templates/.claude/ # Embedded template files
├── internal/
│   ├── cli/                  # CLI commands (init, version, root)
│   └── templates/            # Template loader implementations
│       ├── loader.go         # Loader interface
│       ├── local.go          # Local filesystem loader
│       ├── github.go         # GitHub releases loader
│       └── embedded.go       # Embedded templates loader
├── scripts/
│   ├── pre-commit            # Pre-commit hook script
│   └── install-git-hooks.sh  # Hook installation script
├── .goreleaser.yaml          # GoReleaser configuration
├── Makefile                  # Build automation
├── go.mod                    # Go module definition
└── README.md                 # Project documentation
```

## Commit Guidelines

- Write clear, descriptive commit messages
- Use conventional commit format when possible (e.g., `feat:`, `fix:`, `docs:`)
- Keep commits focused on a single change
- Reference issues/PRs when relevant

### Commit Message Format

```
<type>: <short description>

<longer description if needed>

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

Common types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

## Pull Request Process

1. **Create a feature branch** from `main`
2. **Make your changes** and ensure all checks pass
3. **Push to your fork** and create a pull request
4. **Address review feedback** if any
5. **Wait for CI checks** to pass
6. **Merge** once approved

## Testing Template Loading

The CLI supports multiple template sources for testing:

```bash
# Use local templates
DRAFT_TEMPLATES=/path/to/templates ./bin/draft init

# Use specific GitHub release version
./bin/draft init --version v1.0.0

# Normal use (latest release or embedded fallback)
./bin/draft init
```

## Getting Help

- **Issues**: Report bugs or request features via [GitHub Issues](https://github.com/heiko-braun/draft/issues)
- **Discussions**: Ask questions in [GitHub Discussions](https://github.com/heiko-braun/draft/discussions)

## License

By contributing, you agree that your contributions will be licensed under the same license as the project.
