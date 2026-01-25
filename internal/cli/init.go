package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var forceFlag bool

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [directory]",
		Short: "Initialize Claude spec-driven development workflow",
		Long:  `Initialize a repository with Claude spec-driven development commands and templates.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  runInit,
	}

	cmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Overwrite existing files")

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
	filesCreated, filesOverwritten, err := copyTemplates(absTarget, conflicts)
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
		".claude/commands/plan.md",
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

func copyTemplates(targetDir string, conflicts []string) (int, []string, error) {
	filesCreated := 0
	var filesOverwritten []string

	// Walk the embedded filesystem
	err := fs.WalkDir(templateFS, "templates/.claude", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory
		if path == "templates/.claude" {
			return nil
		}

		// Calculate relative path from templates/.claude
		relPath := strings.TrimPrefix(path, "templates/")
		targetPath := filepath.Join(targetDir, relPath)

		if d.IsDir() {
			// Create directory
			return os.MkdirAll(targetPath, 0755)
		}

		// Read file from embedded FS
		content, err := fs.ReadFile(templateFS, path)
		if err != nil {
			return fmt.Errorf("failed to read embedded file %s: %w", path, err)
		}

		// Check if file exists
		_, err = os.Stat(targetPath)
		fileExists := err == nil

		// Write file
		if err := os.WriteFile(targetPath, content, 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", targetPath, err)
		}

		if fileExists && forceFlag {
			filesOverwritten = append(filesOverwritten, relPath)
		} else {
			filesCreated++
		}

		return nil
	})

	return filesCreated, filesOverwritten, err
}
