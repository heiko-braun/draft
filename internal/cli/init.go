package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/heiko-braun/draft/internal/templates"
	"github.com/spf13/cobra"
)

var (
	forceFlag   bool
	versionFlag string
	agentFlag   string
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [directory]",
		Short: "Initialize specification-driven development workflow",
		Long:  `Initialize a repository with specification-driven development commands and templates for AI coding assistants.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  runInit,
	}

	cmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Overwrite existing files")
	cmd.Flags().StringVar(&versionFlag, "version", "", "Fetch templates from specific GitHub release version (e.g., v1.0.0)")
	cmd.Flags().StringVar(&agentFlag, "agent", "", "AI agent to initialize for: 'claude', 'cursor', or both if omitted")

	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	targetDir := "."
	if len(args) > 0 {
		targetDir = args[0]
	}

	// Validate agent flag
	if agentFlag != "" && agentFlag != "claude" && agentFlag != "cursor" {
		return fmt.Errorf("invalid --agent value: %s (must be 'claude' or 'cursor')", agentFlag)
	}

	// Convert to absolute path
	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to resolve target directory: %w", err)
	}

	// Check if directory exists and is writable
	if err := checkWritable(absTarget); err != nil {
		return err
	}

	// Load templates using priority: local env var > GitHub > embedded
	result, err := loadTemplates()
	if err != nil {
		return err
	}

	// Display template source
	fmt.Printf("Loading templates from %s\n", result.Source)

	// Determine which agents to initialize
	agents := getAgentsToInit()

	// Scan for conflicts
	conflicts, err := findConflicts(absTarget, agents)
	if err != nil {
		return fmt.Errorf("failed to scan for conflicts: %w", err)
	}

	// If conflicts exist and no force flag, warn and exit
	if len(conflicts) > 0 && !forceFlag {
		fmt.Fprintln(os.Stderr, "The following files already exist:")
		for _, conflict := range conflicts {
			fmt.Fprintf(os.Stderr, "  - %s\n", conflict)
		}
		fmt.Fprintln(os.Stderr, "\nUse --force to overwrite existing files")
		return fmt.Errorf("files already exist")
	}

	// Copy files
	filesCreated, filesOverwritten, err := copyTemplates(absTarget, conflicts, result.FS, agents)
	if err != nil {
		return fmt.Errorf("failed to copy templates: %w", err)
	}

	// Display summary
	if forceFlag && len(filesOverwritten) > 0 {
		fmt.Printf("Overwritten %d files:\n", len(filesOverwritten))
		for _, file := range filesOverwritten {
			fmt.Printf("  - %s\n", file)
		}
	}

	total := filesCreated + len(filesOverwritten)

	// Build message for which directories were created
	dirs := []string{}
	for _, agent := range agents {
		switch agent {
		case "claude":
			dirs = append(dirs, ".claude/")
		case "cursor":
			dirs = append(dirs, ".cursor/")
		}
	}
	dirs = append(dirs, "specs/")

	dirsMsg := ""
	if len(dirs) == 1 {
		dirsMsg = dirs[0]
	} else if len(dirs) == 2 {
		dirsMsg = dirs[0] + " and " + dirs[1]
	} else {
		dirsMsg = dirs[0] + ", " + dirs[1] + " and " + dirs[2]
	}

	fmt.Printf("Successfully created %d files in %s\n", total, dirsMsg)

	return nil
}

// getAgentsToInit returns which agents to initialize based on --agent flag
func getAgentsToInit() []string {
	switch agentFlag {
	case "claude":
		return []string{"claude"}
	case "cursor":
		return []string{"cursor"}
	default:
		return []string{"claude", "cursor"}
	}
}

// loadTemplates attempts to load templates with priority: local > GitHub > embedded
func loadTemplates() (*templates.LoadResult, error) {
	// 1. Check for DRAFT_TEMPLATES environment variable
	if localPath := os.Getenv("DRAFT_TEMPLATES"); localPath != "" {
		loader := templates.NewLocalLoader(localPath)
		templatesFS, err := loader.Load()
		if err != nil {
			return nil, err
		}
		return &templates.LoadResult{
			FS:     templatesFS,
			Source: loader.Source(),
		}, nil
	}

	// 2. Try GitHub loader
	githubLoader := templates.NewGitHubLoader("", "", versionFlag)
	templatesFS, err := githubLoader.Load()
	if err != nil {
		// GitHub failed, fall back to embedded
		fmt.Fprintf(os.Stderr, "Warning: Failed to fetch templates from GitHub (%v), using embedded templates\n", err)
		embeddedLoader := templates.NewEmbeddedLoader(templateFS, "templates")
		templatesFS, err = embeddedLoader.Load()
		if err != nil {
			return nil, fmt.Errorf("failed to load embedded templates: %w", err)
		}
		return &templates.LoadResult{
			FS:     templatesFS,
			Source: embeddedLoader.Source(),
		}, nil
	}

	return &templates.LoadResult{
		FS:     templatesFS,
		Source: githubLoader.Source(),
	}, nil
}

func checkWritable(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory does not exist: %s", dir)
		}
		return fmt.Errorf("failed to access directory: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", dir)
	}

	// Try to create a temp file to check write permissions
	testFile := filepath.Join(dir, ".claude-write-test")
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("directory is not writable: %w", err)
	}
	f.Close()
	os.Remove(testFile)

	return nil
}

func findConflicts(targetDir string, agents []string) ([]string, error) {
	var conflicts []string

	// Build list of files to check based on agents
	filesToCheck := []string{}

	for _, agent := range agents {
		switch agent {
		case "claude":
			filesToCheck = append(filesToCheck,
				".claude/commands/spec.md",
				".claude/commands/implement.md",
				".claude/commands/refine.md",
				".claude/commands/verify-spec.md",
			)
		case "cursor":
			filesToCheck = append(filesToCheck,
				".cursor/skills/spec/SKILL.md",
				".cursor/skills/implement/SKILL.md",
				".cursor/skills/refine/SKILL.md",
				".cursor/skills/verify-spec/SKILL.md",
			)
		}
	}

	// specs/TEMPLATE.md is always checked
	filesToCheck = append(filesToCheck, "specs/TEMPLATE.md")

	for _, file := range filesToCheck {
		fullPath := filepath.Join(targetDir, file)
		if _, err := os.Stat(fullPath); err == nil {
			conflicts = append(conflicts, file)
		}
	}

	return conflicts, nil
}

func copyTemplates(targetDir string, conflicts []string, templatesFS fs.FS, agents []string) (int, []string, error) {
	filesCreated := 0
	var filesOverwritten []string

	// Build list of roots to copy based on agents
	roots := []string{"specs"} // Always include specs

	for _, agent := range agents {
		switch agent {
		case "claude":
			roots = append(roots, ".claude")
		case "cursor":
			roots = append(roots, ".cursor")
		}
	}

	for _, root := range roots {
		err := fs.WalkDir(templatesFS, root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Skip the root directory
			if path == root {
				return nil
			}

			// Calculate target path
			targetPath := filepath.Join(targetDir, path)

			if d.IsDir() {
				// Create directory
				return os.MkdirAll(targetPath, 0755)
			}

			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", targetPath, err)
			}

			// Read file from FS
			content, err := fs.ReadFile(templatesFS, path)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", path, err)
			}

			// Check if file exists
			_, err = os.Stat(targetPath)
			fileExists := err == nil

			// Write file
			if err := os.WriteFile(targetPath, content, 0644); err != nil {
				return fmt.Errorf("failed to write file %s: %w", targetPath, err)
			}

			if fileExists && forceFlag {
				filesOverwritten = append(filesOverwritten, path)
			} else {
				filesCreated++
			}

			return nil
		})
		if err != nil {
			// If the root directory doesn't exist in the FS, skip it gracefully
			// This happens when loading from GitHub releases that don't have all formats yet
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return filesCreated, filesOverwritten, err
		}
	}

	return filesCreated, filesOverwritten, nil
}
