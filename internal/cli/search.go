package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/heiko-braun/draft/internal/search"
	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	var (
		limitFlag  int
		statusFlag bool
		dbFlag     string
	)

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the project index",
		Long:  `Search the FTS5 index for relevant files using natural language or substring queries.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if statusFlag {
				return runSearchStatus(dbFlag)
			}
			if len(args) == 0 {
				return fmt.Errorf("query required: draft search <query>")
			}
			query := strings.Join(args, " ")
			return runSearch(query, limitFlag, dbFlag)
		},
	}

	cmd.Flags().IntVar(&limitFlag, "limit", 20, "Maximum number of results")
	cmd.Flags().BoolVar(&statusFlag, "status", false, "Show index status")
	cmd.Flags().StringVar(&dbFlag, "db", "", "Override index database path")

	return cmd
}

func runSearch(query string, limit int, dbOverride string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	root, err := search.DetectProjectRoot(cwd)
	if err != nil {
		return err
	}

	dbPath, err := search.IndexPath(root, dbOverride)
	if err != nil {
		return err
	}

	store, err := search.OpenStore(dbPath, root)
	if err != nil {
		return fmt.Errorf("open index: %w (run 'draft index' first)", err)
	}
	defer store.Close()

	results, err := search.Search(store, query, limit)
	if err != nil {
		return err
	}

	fmt.Print(search.FormatResults(results))
	return nil
}

func runSearchStatus(dbOverride string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	root, err := search.DetectProjectRoot(cwd)
	if err != nil {
		return err
	}

	dbPath, err := search.IndexPath(root, dbOverride)
	if err != nil {
		return err
	}

	store, err := search.OpenStore(dbPath, root)
	if err != nil {
		return fmt.Errorf("no index found (run 'draft index' first)")
	}
	defer store.Close()

	fileCount, _ := store.FileCount()
	lastIndexed, _ := store.Meta("created_at")

	fi, err := os.Stat(dbPath)
	var sizeStr string
	if err == nil {
		sizeStr = formatSize(fi.Size())
	} else {
		sizeStr = "unknown"
	}

	fmt.Printf("Index:   %s\n", dbPath)
	fmt.Printf("Project: %s\n", root)
	fmt.Printf("Files indexed: %d\n", fileCount)
	fmt.Printf("Last indexed:  %s\n", lastIndexed)
	fmt.Printf("Database size: %s\n", sizeStr)

	return nil
}
