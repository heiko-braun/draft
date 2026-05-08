package cli

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/heiko-braun/draft/internal/review"
	"github.com/spf13/cobra"
)

func newReviewCmd() *cobra.Command {
	var (
		port       int
		branch     string
		syncFlag   bool
		statusFlag bool
		debugFlag  bool
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
			return runReview(port, branch, syncFlag, statusFlag, debugFlag, args)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8787, "Port for the review server")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Source branch for documents (overrides config)")
	cmd.Flags().BoolVar(&syncFlag, "sync", false, "Sync review data without opening UI")
	cmd.Flags().BoolVar(&statusFlag, "status", false, "Print review status and exit")
	cmd.Flags().BoolVar(&debugFlag, "debug", false, "Enable debug logging for the review server")

	return cmd
}

func runReview(port int, branchOverride string, syncOnly, statusOnly, debug bool, args []string) error {
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

	// 4. Source branch: default to the currently checked-out branch so the UI
	// reflects what the user is actually working on.
	sourceBranch := currentGitBranch(repo.Root)
	if sourceBranch == "" {
		sourceBranch = cfg.DefaultBranch
	}
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
		}
	}

	// 7. Create syncer and store.
	syncer := review.NewSyncer(repo.Root, wt.ReviewsPath, wt.DocsPath, sourceBranch)
	store := review.NewStore(wt.ReviewsPath)

	// 8. --status and --sync need sync first.
	if statusOnly || syncOnly {
		if syncErr := syncer.SyncAll(); syncErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: sync failed: %v\n", syncErr)
		}
		if statusOnly {
			return printReviewStatus(store, syncer)
		}
		fmt.Println("Sync complete")
		return nil
	}

	// 9. For the UI path, index documents and start server immediately.
	// Sync runs in the background — no need to block on network I/O.
	docsRoot := repo.Root
	if branchOverride != "" {
		docsRoot = wt.DocsPath
	}

	docIndex, err := review.IndexDocuments(docsRoot, cfg.DocumentPaths)
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
		docsRoot, wt.ReviewsPath, repo.Root,
		sourceBranch, repoName, userEmail,
		debug,
	)

	addr := fmt.Sprintf("localhost:%d", port)
	url := fmt.Sprintf("http://%s", addr)

	// If a file argument is provided, add it as a URL fragment.
	if len(args) > 0 {
		url = url + "#" + args[0]
	}

	fmt.Printf("Server running at %s\n", url)
	fmt.Println("Press Ctrl+C to stop")

	// Open browser immediately.
	go openBrowser(url)

	// Sync in background.
	go func() {
		if syncErr := syncer.SyncAll(); syncErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: sync failed: %v\n", syncErr)
		}
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

// currentGitBranch returns the currently checked-out branch name, or "" on error.
func currentGitBranch(repoRoot string) string {
	cmd := exec.Command("git", "-C", repoRoot, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
