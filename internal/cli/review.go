package cli

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/heiko-braun/draft/internal/review"
	"github.com/spf13/cobra"
)

func newReviewCmd() *cobra.Command {
	var (
		port       int
		branch     string
		syncFlag   bool
		statusFlag bool
	)

	cmd := &cobra.Command{
		Use:   "review [file]",
		Short: "Launch the document review UI",
		Long: `Review opens an interactive browser UI for document review with inline
annotations, threaded discussions, and publishing.

Examples:
  draft review
  draft review --branch feature/x
  draft review specs/auth.md
  draft review --sync
  draft review --status`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReview(port, branch, syncFlag, statusFlag, args)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8787, "Port for the review server")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Source branch for documents (overrides config)")
	cmd.Flags().BoolVar(&syncFlag, "sync", false, "Sync review data without opening UI")
	cmd.Flags().BoolVar(&statusFlag, "status", false, "Print review status and exit")

	return cmd
}

func runReview(port int, branchOverride string, syncOnly, statusOnly bool, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// 1. Detect repo.
	repo, err := review.DetectRepo(cwd)
	if err != nil {
		return fmt.Errorf("not a git repository (or no remote configured): %w", err)
	}

	// 2. Initialize review branch if needed.
	if err := review.InitReviewBranch(repo.Root); err != nil {
		return fmt.Errorf("initializing review branch: %w", err)
	}

	// 3. Default config.
	cfg := review.DefaultConfig()

	// 4. Source branch.
	sourceBranch := cfg.DefaultBranch
	if branchOverride != "" {
		sourceBranch = branchOverride
	}

	// 5. Ensure worktrees.
	wt, err := review.EnsureWorktrees(repo.Root, cfg, sourceBranch)
	if err != nil {
		return fmt.Errorf("setting up worktrees: %w", err)
	}

	// 6. Read config from reviews worktree if it exists.
	configPath := filepath.Join(wt.ReviewsPath, "config.json")
	if data, readErr := os.ReadFile(configPath); readErr == nil {
		if parsed, parseErr := review.ParseConfig(data); parseErr == nil {
			cfg = parsed
			// Re-apply branch override (config may have changed default).
			if branchOverride == "" {
				sourceBranch = cfg.DefaultBranch
			}
		}
	}

	// 7. Sync (non-fatal).
	syncer := review.NewSyncer(repo.Root, wt.ReviewsPath, wt.DocsPath, sourceBranch)
	if syncErr := syncer.SyncAll(); syncErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: sync failed: %v\n", syncErr)
	}

	store := review.NewStore(wt.ReviewsPath)

	// 8. --status: print status and exit.
	if statusOnly {
		return printReviewStatus(store, syncer)
	}

	// 9. --sync: just sync and exit.
	if syncOnly {
		fmt.Println("Sync complete")
		return nil
	}

	// 10. Index documents.
	docIndex, err := review.IndexDocuments(wt.DocsPath, cfg.DocumentPaths)
	if err != nil {
		return fmt.Errorf("indexing documents: %w", err)
	}

	// Determine user email for authoring comments.
	userEmail := ""
	if p, pErr := store.EnsureParticipantFromGit(); pErr == nil {
		userEmail = p.Email
	}

	// Determine repo name for display.
	repoName := filepath.Base(repo.Root)

	// Create server.
	srv := review.NewServer(
		store, syncer, docIndex, cfg,
		wt.DocsPath, wt.ReviewsPath, repo.Root,
		sourceBranch, repoName, userEmail,
	)

	addr := fmt.Sprintf("localhost:%d", port)
	url := fmt.Sprintf("http://%s", addr)

	// If a file argument is provided, add it as a URL fragment.
	if len(args) > 0 {
		url = url + "#" + args[0]
	}

	fmt.Printf("Starting review server...\n")
	fmt.Printf("Server running at %s\n", url)
	fmt.Println("Press Ctrl+C to stop")

	// Open browser after a short delay.
	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser(url)
	}()

	if err := http.ListenAndServe(addr, srv); err != nil {
		return fmt.Errorf("server failed: %w", err)
	}

	return nil
}

func printReviewStatus(store *review.Store, syncer *review.Syncer) error {
	openReviews, err := store.ListOpenReviews()
	if err != nil {
		return fmt.Errorf("listing open reviews: %w", err)
	}

	allThreads, err := store.ListAllThreads()
	if err != nil {
		return fmt.Errorf("listing threads: %w", err)
	}

	openThreads := 0
	for _, t := range allThreads {
		if t.Status == review.ThreadOpen {
			openThreads++
		}
	}

	pending, _ := syncer.HasPendingChanges()

	fmt.Printf("Open reviews:    %d\n", len(openReviews))
	fmt.Printf("Open threads:    %d\n", openThreads)
	fmt.Printf("Total threads:   %d\n", len(allThreads))
	if pending {
		fmt.Printf("Pending changes: yes\n")
	} else {
		fmt.Printf("Pending changes: no\n")
	}

	return nil
}
