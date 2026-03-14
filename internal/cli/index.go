package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/heiko-braun/draft/internal/search"
	"github.com/spf13/cobra"
)

func newIndexCmd() *cobra.Command {
	var (
		forceIndex bool
		pruneFlag  bool
		listFlag   bool
		dbFlag     string
	)

	cmd := &cobra.Command{
		Use:   "index",
		Short: "Build or update the project search index",
		Long:  `Walks the project tree and maintains an FTS5 search index for fast, ranked code search.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if listFlag {
				return runIndexList()
			}
			if pruneFlag {
				return runIndexPrune()
			}
			return runIndex(forceIndex, dbFlag)
		},
	}

	cmd.Flags().BoolVar(&forceIndex, "force", false, "Drop and rebuild the index from scratch")
	cmd.Flags().BoolVar(&pruneFlag, "prune", false, "Delete indexes for projects that no longer exist")
	cmd.Flags().BoolVar(&listFlag, "list", false, "List all known indexes")
	cmd.Flags().StringVar(&dbFlag, "db", "", "Override index database path")

	return cmd
}

func runIndex(force bool, dbOverride string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}

	root, err := search.DetectProjectRoot(cwd)
	if err != nil {
		return fmt.Errorf("detect project root: %w", err)
	}

	dbPath, err := search.IndexPath(root, dbOverride)
	if err != nil {
		return fmt.Errorf("index path: %w", err)
	}

	store, err := search.OpenStore(dbPath, root)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	start := time.Now()
	result, err := search.Index(store, root, force)
	if err != nil {
		return fmt.Errorf("index: %w", err)
	}
	elapsed := time.Since(start)

	fmt.Printf("Indexed %s in %s\n", root, elapsed.Round(time.Millisecond))
	fmt.Printf("  files indexed: %d, unchanged: %d, deleted: %d, skipped: %d\n",
		result.FilesIndexed, result.FilesUnchanged, result.FilesDeleted, result.FilesSkipped)

	return nil
}

func runIndexList() error {
	infos, err := search.ListIndexes()
	if err != nil {
		return err
	}

	if len(infos) == 0 {
		fmt.Println("No indexes found.")
		return nil
	}

	fmt.Printf("%-50s %6s %10s %s\n", "PROJECT", "FILES", "SIZE", "LAST INDEXED")
	for _, info := range infos {
		size := formatSize(info.SizeBytes)
		fmt.Printf("%-50s %6d %10s %s\n", info.ProjectRoot, info.FileCount, size, info.LastIndexed)
	}

	return nil
}

func runIndexPrune() error {
	pruned, err := search.PruneIndexes()
	if err != nil {
		return err
	}

	if len(pruned) == 0 {
		fmt.Println("No orphaned indexes found.")
		return nil
	}

	fmt.Printf("Pruned %d orphaned index(es):\n", len(pruned))
	for _, root := range pruned {
		fmt.Printf("  - %s\n", root)
	}

	return nil
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
