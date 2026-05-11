package cli

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/heiko-braun/draft/internal/review"
	"github.com/spf13/cobra"
)

func newReviewCmd() *cobra.Command {
	var (
		port       int
		branch     string
		server     string
		statusFlag bool
		debugFlag  bool
	)

	cmd := &cobra.Command{
		Use:   "review [file]",
		Short: "Launch the document review UI",
		Long: `Review opens an interactive browser UI for document review with inline
annotations, threaded discussions, and remote storage via the reviewd service.

The review data is stored on a remote reviewd server (default: https://reviewd-dev.up.railway.app).
Use --server or REVIEWD_URL to point to a different server.

Examples:
  draft review
  draft review --server https://reviews.example.com
  draft review --branch feature/x
  draft review specs/auth.md
  draft review --status`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReview(port, branch, server, statusFlag, debugFlag, args)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8787, "Port for the local review server")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Source branch for documents (overrides config)")
	cmd.Flags().StringVarP(&server, "server", "s", "", "URL of the reviewd service (default: REVIEWD_URL or http://localhost:5100)")
	cmd.Flags().BoolVar(&statusFlag, "status", false, "Print review status and exit")
	cmd.Flags().BoolVar(&debugFlag, "debug", false, "Enable debug logging for the review server")

	return cmd
}

func runReview(port int, branchOverride, serverURL string, statusOnly, debug bool, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// 1. Detect repo.
	repo, err := review.DetectRepo(cwd)
	if err != nil {
		return fmt.Errorf("not a git repository (or no remote configured): %w", err)
	}

	// 2. Determine source branch.
	cfg := review.DefaultConfig()
	sourceBranch := currentGitBranch(repo.Root)
	if sourceBranch == "" {
		sourceBranch = cfg.DefaultBranch
	}
	if branchOverride != "" {
		sourceBranch = branchOverride
	}

	// 3. Get GitHub token for reviewd authentication.
	token := githubToken()

	// 4. Create remote client.
	reviewdURL := serverURL
	if reviewdURL == "" {
		reviewdURL = os.Getenv("REVIEWD_URL")
	}
	if reviewdURL == "" {
		reviewdURL = "https://reviewd-dev.up.railway.app"
	}

	owner, repoName := repo.OwnerRepo()
	client := review.NewClient(reviewdURL, owner, repoName, token)

	// 5. --status mode.
	if statusOnly {
		return printReviewStatus(client, client)
	}

	// 6. Index documents from local filesystem.
	docIndex, err := review.IndexDocuments(repo.Root, cfg.DocumentPaths)
	if err != nil {
		return fmt.Errorf("indexing documents: %w", err)
	}

	// 7. Determine user email.
	userEmail := gitConfigValue("user.email")

	// 8. Create local server backed by remote client.
	srv := review.NewServer(
		client, client, docIndex, cfg,
		repo.Root, repo.Root,
		sourceBranch, filepath.Base(repo.Root), userEmail,
		debug,
	)

	addr := fmt.Sprintf("localhost:%d", port)
	url := fmt.Sprintf("http://%s", addr)

	if len(args) > 0 {
		url = url + "#" + args[0]
	}

	fmt.Printf("Server running at %s\n", url)
	fmt.Printf("Connected to reviewd at %s\n", reviewdURL)
	fmt.Println("Press Ctrl+C to stop")

	go openBrowser(url)

	if err := http.ListenAndServe(addr, srv); err != nil {
		return fmt.Errorf("server failed: %w", err)
	}

	return nil
}

func printReviewStatus(store review.ReviewStore, syncer review.ReviewSyncer) error {
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

// githubToken returns a GitHub token from the environment or gh CLI.
func githubToken() string {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}

// gitConfigValue returns a git config value, or "" on error.
func gitConfigValue(key string) string {
	out, err := exec.Command("git", "config", "--get", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// openBrowser opens a URL in the user's default browser.
func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not open browser: %v\n", err)
	}
}
