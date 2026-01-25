package cli

import (
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

	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	targetDir := "."
	if len(args) > 0 {
		targetDir = args[0]
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

	// Scan for conflicts
	conflicts, err := findConflicts(absTarget)
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
	filesCreated, filesOverwritten, err := copyTemplates(absTarget, conflicts, result.FS)
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
	fmt.Printf("Successfully created %d files in .claude/\n", total)

	return nil
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

func findConflicts(targetDir string) ([]string, error) {
	var conflicts []string

	// Check for existing files
	filesToCheck := []string{
		".claude/commands/spec.md",
		".claude/commands/implement.md",
		".claude/commands/refine.md",
		".claude/specs/TEMPLATE.md",
	}

	for _, file := range filesToCheck {
		fullPath := filepath.Join(targetDir, file)
		if _, err := os.Stat(fullPath); err == nil {
			conflicts = append(conflicts, file)
		}
	}

	return conflicts, nil
}

func copyTemplates(targetDir string, conflicts []string, templatesFS fs.FS) (int, []string, error) {
	filesCreated := 0
	var filesOverwritten []string

	// Walk the filesystem starting at .claude
	err := fs.WalkDir(templatesFS, ".claude", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory
		if path == ".claude" {
			return nil
		}

		// Calculate target path
		targetPath := filepath.Join(targetDir, path)

		if d.IsDir() {
			// Create directory
			return os.MkdirAll(targetPath, 0755)
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

	return filesCreated, filesOverwritten, err
}
