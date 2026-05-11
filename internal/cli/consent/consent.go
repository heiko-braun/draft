// Package consent manages user consent for sending data to remote services.
package consent

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Status represents the user's consent decision.
type Status int

const (
	Unknown Status = iota // no consent file found
	Granted               // user accepted
	Denied                // user declined
)

const (
	consentFile = "consent"
	consentKey  = "review_data"
	dirName     = "draft"
)

// configDir returns ~/.config/draft, respecting XDG_CONFIG_HOME.
func configDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, dirName)
}

// filePath returns the full path to the consent file.
func filePath() string {
	return filepath.Join(configDir(), consentFile)
}

// Read returns the current consent status.
func Read() Status {
	return ReadFrom(filePath())
}

// ReadFrom returns the consent status from a specific file path.
func ReadFrom(path string) Status {
	data, err := os.ReadFile(path)
	if err != nil {
		return Unknown
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if key == consentKey {
			if val == "true" {
				return Granted
			}
			return Denied
		}
	}
	return Unknown
}

// Write persists the consent decision.
func Write(granted bool) error {
	return WriteTo(filePath(), granted)
}

// WriteTo persists the consent decision to a specific file path.
func WriteTo(path string, granted bool) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	val := "false"
	if granted {
		val = "true"
	}
	content := fmt.Sprintf("# draft consent settings\nreview_data = %s\n", val)
	return os.WriteFile(path, []byte(content), 0644)
}

const noticeText = `draft review sends data to a remote reviewd service.

The following information is transmitted:
  - Repository owner and name
  - Document paths and review comments
  - Your identity (GitHub token)

Target: %s
`

// CheckOrPrompt verifies consent, prompting interactively if needed.
// It reads from r and writes prompts/notices to w.
// Returns nil if consent is granted, an error otherwise.
func CheckOrPrompt(defaultURL string, r io.Reader, w io.Writer) error {
	return checkOrPromptAt(filePath(), defaultURL, r, w)
}

func checkOrPromptAt(path, defaultURL string, r io.Reader, w io.Writer) error {
	status := ReadFrom(path)

	switch status {
	case Granted:
		return nil
	case Denied:
		return fmt.Errorf("data collection consent not granted; edit %s to change", path)
	}

	// Unknown — prompt the user.
	fmt.Fprintf(w, noticeText, defaultURL)
	fmt.Fprint(w, "Do you consent to sending this data? [Y/n] ")

	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return fmt.Errorf("no input received; consent not granted")
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))

	granted := answer == "" || answer == "y" || answer == "yes"
	if err := WriteTo(path, granted); err != nil {
		return fmt.Errorf("saving consent: %w", err)
	}
	if !granted {
		return fmt.Errorf("data collection consent declined; to change, edit %s", path)
	}
	return nil
}
