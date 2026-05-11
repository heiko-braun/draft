package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const (
	githubRepo       = "heiko-braun/draft"
	installScriptURL = "https://raw.githubusercontent.com/" + githubRepo + "/main/install.sh"
)

func newVersionCmd() *cobra.Command {
	var check, update bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			if update {
				return runUpdate()
			}
			if check {
				return runCheck()
			}
			fmt.Printf("draft version %s\n", appVersion)
			return nil
		},
	}

	cmd.Flags().BoolVar(&check, "check", false, "Check if a newer version is available")
	cmd.Flags().BoolVar(&update, "update", false, "Update draft to the latest version")

	return cmd
}

func runCheck() error {
	latest, err := fetchLatestVersion()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	current := parseSemver(appVersion)
	remote := parseSemver(latest)

	if remote.greaterThan(current) {
		fmt.Printf("Update available: %s → %s\n", appVersion, latest)
		fmt.Println("Run `draft version --update` to install it.")
	} else {
		fmt.Printf("You're up to date (current: %s, latest: %s)\n", appVersion, latest)
	}
	return nil
}

func runUpdate() error {
	latest, err := fetchLatestVersion()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	current := parseSemver(appVersion)
	remote := parseSemver(latest)

	if !remote.greaterThan(current) {
		fmt.Printf("You're up to date (current: %s, latest: %s)\n", appVersion, latest)
		return nil
	}

	fmt.Printf("Updating draft: %s → %s\n", appVersion, latest)

	script := fmt.Sprintf("curl -fsSL %s | DRAFT_VERSION=%s bash", installScriptURL, latest)
	cmd := exec.Command("bash", "-c", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	fmt.Println("Update complete.")
	return nil
}

type ghRelease struct {
	TagName string `json:"tag_name"`
}

func fetchLatestVersion() (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	if release.TagName == "" {
		return "", fmt.Errorf("no tag_name in GitHub response")
	}
	return release.TagName, nil
}

type semver struct {
	major, minor, patch int
}

func (a semver) greaterThan(b semver) bool {
	if a.major != b.major {
		return a.major > b.major
	}
	if a.minor != b.minor {
		return a.minor > b.minor
	}
	return a.patch > b.patch
}

// parseSemver extracts major.minor.patch from version strings like
// "v0.5.5", "0.5.5", "v0.5.5-28-gb603f04-dirty", or "dev".
func parseSemver(v string) semver {
	v = strings.TrimPrefix(v, "v")
	// Strip anything after a hyphen (git-describe suffix)
	if idx := strings.Index(v, "-"); idx != -1 {
		v = v[:idx]
	}
	parts := strings.Split(v, ".")
	s := semver{}
	if len(parts) >= 1 {
		s.major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) >= 2 {
		s.minor, _ = strconv.Atoi(parts[1])
	}
	if len(parts) >= 3 {
		s.patch, _ = strconv.Atoi(parts[2])
	}
	return s
}
