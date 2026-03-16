package templates

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"testing"
)

// TestGitHubLoaderAllowedFiles verifies that all expected template files
// are included in the allowedFiles list and can be extracted from tarballs
func TestGitHubLoaderAllowedFiles(t *testing.T) {
	tests := []struct {
		name          string
		filePath      string
		shouldExtract bool
	}{
		// Claude commands - should all be extracted
		{"claude spec command", "heiko-braun-draft-abc123/.claude/commands/spec.md", true},
		{"claude implement command", "heiko-braun-draft-abc123/.claude/commands/implement.md", true},
		{"claude refine command", "heiko-braun-draft-abc123/.claude/commands/refine.md", true},
		{"claude verify command", "heiko-braun-draft-abc123/.claude/commands/verify.md", true},
		{"claude verify agent", "heiko-braun-draft-abc123/.claude/agents/verify-agent.md", true},

		// Cursor skills - should all be extracted
		{"cursor spec skill", "heiko-braun-draft-abc123/.cursor/skills/spec/SKILL.md", true},
		{"cursor implement skill", "heiko-braun-draft-abc123/.cursor/skills/implement/SKILL.md", true},
		{"cursor refine skill", "heiko-braun-draft-abc123/.cursor/skills/refine/SKILL.md", true},
		{"cursor verify skill", "heiko-braun-draft-abc123/.cursor/skills/verify/SKILL.md", true},
		{"cursor verify agent", "heiko-braun-draft-abc123/.cursor/agents/verify-agent.md", true},

		// Template files - should be extracted
		{"specs TEMPLATE.md", "heiko-braun-draft-abc123/specs/TEMPLATE.md", true},
		{"cursor specs TEMPLATE.md", "heiko-braun-draft-abc123/.cursor/specs/TEMPLATE.md", true},
		{"old claude specs TEMPLATE.md", "heiko-braun-draft-abc123/.claude/specs/TEMPLATE.md", true},

		// Principles files - should be extracted
		{"design principles", "heiko-braun-draft-abc123/.principles/design-principles.md", true},
		{"review role", "heiko-braun-draft-abc123/.principles/review-role.md", true},

		// Project-specific files - should NOT be extracted
		{"project spec file", "heiko-braun-draft-abc123/specs/my-feature.md", false},
		{"random file", "heiko-braun-draft-abc123/README.md", false},
		{"source code", "heiko-braun-draft-abc123/cmd/draft/main.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a tarball with the test file
			tarballData := createTestTarball(t, tt.filePath, "test content")

			// Extract using the GitHub loader's logic
			extractedFS, err := extractFromTarball(tarballData)
			if err != nil {
				t.Fatalf("failed to extract tarball: %v", err)
			}

			// Determine the expected path after extraction
			// (stripping GitHub's root directory prefix)
			expectedPath := stripGitHubRoot(tt.filePath)
			if expectedPath == ".claude/specs/TEMPLATE.md" {
				// Old location gets normalized to new location
				expectedPath = "specs/TEMPLATE.md"
			}

			// Check if file exists in extracted FS
			_, err = fs.Stat(extractedFS, expectedPath)
			fileExists := err == nil

			if tt.shouldExtract && !fileExists {
				t.Errorf("expected file %q to be extracted, but it was not found", expectedPath)
			}
			if !tt.shouldExtract && fileExists {
				t.Errorf("expected file %q to NOT be extracted, but it was found", expectedPath)
			}
		})
	}
}

// TestVerifySpecIncluded specifically tests that verify-spec is in the allowed list
// This test would have caught the bug where verify-spec was missing
func TestVerifySpecIncluded(t *testing.T) {
	requiredFiles := []string{
		".claude/commands/verify.md",
		".claude/agents/verify-agent.md",
		".cursor/skills/verify/SKILL.md",
		".cursor/agents/verify-agent.md",
	}

	for _, filePath := range requiredFiles {
		t.Run(filePath, func(t *testing.T) {
			// Create tarball with verify-spec file
			fullPath := "heiko-braun-draft-abc123/" + filePath
			tarballData := createTestTarball(t, fullPath, "# Verify Spec\nTest content")

			// Extract using GitHub loader
			extractedFS, err := extractFromTarball(tarballData)
			if err != nil {
				t.Fatalf("failed to extract tarball: %v", err)
			}

			// Check if verify-spec was extracted
			_, err = fs.Stat(extractedFS, filePath)
			if err != nil {
				t.Errorf("verify-spec file %q was not extracted from tarball - it may be missing from allowedFiles list", filePath)
			}
		})
	}
}

// Helper functions

// stripGitHubRoot removes the GitHub root directory from a path
// e.g., "owner-repo-sha/path/file.md" -> "path/file.md"
func stripGitHubRoot(path string) string {
	parts := bytes.SplitN([]byte(path), []byte("/"), 2)
	if len(parts) < 2 {
		return ""
	}
	return string(parts[1])
}

// createTestTarball creates a gzipped tarball with a single file
func createTestTarball(t *testing.T, filePath, content string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Write file to tarball
	header := &tar.Header{
		Name: filePath,
		Mode: 0644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatalf("failed to write tar header: %v", err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatalf("failed to write tar content: %v", err)
	}

	// Close tar and gzip writers
	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}

	return buf.Bytes()
}

// extractFromTarball mimics the GitHub loader's extraction logic
func extractFromTarball(tarballData []byte) (fs.FS, error) {
	gzr, err := gzip.NewReader(bytes.NewReader(tarballData))
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	memFS := newMemFS()
	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Strip GitHub root directory
		parts := bytes.SplitN([]byte(header.Name), []byte("/"), 2)
		if len(parts) < 2 {
			continue
		}
		relPath := string(parts[1])

		targetPath, allowed := resolveTemplatePath(relPath)
		if !allowed {
			continue
		}

		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}

		memFS.addFile(targetPath, content)
	}

	return memFS, nil
}
