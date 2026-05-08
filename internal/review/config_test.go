package review

import (
	"encoding/json"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if len(cfg.DocumentPaths) != 4 {
		t.Errorf("DocumentPaths length = %d, want 4", len(cfg.DocumentPaths))
	}
	expectedPaths := []string{"specs/", "docs/", "rfcs/", "adrs/"}
	for i, p := range expectedPaths {
		if cfg.DocumentPaths[i] != p {
			t.Errorf("DocumentPaths[%d] = %q, want %q", i, cfg.DocumentPaths[i], p)
		}
	}

	if len(cfg.FilePatterns) != 2 {
		t.Errorf("FilePatterns length = %d, want 2", len(cfg.FilePatterns))
	}
	expectedPatterns := []string{"*.md", "*.mdx"}
	for i, p := range expectedPatterns {
		if cfg.FilePatterns[i] != p {
			t.Errorf("FilePatterns[%d] = %q, want %q", i, cfg.FilePatterns[i], p)
		}
	}

	if cfg.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", cfg.DefaultBranch, "main")
	}
}

func TestMarshalJSON(t *testing.T) {
	cfg := DefaultConfig()
	data, err := cfg.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	// Verify it is valid JSON.
	if !json.Valid(data) {
		t.Fatalf("MarshalJSON produced invalid JSON: %s", data)
	}

	// Verify we can round-trip it.
	parsed, err := ParseConfig(data)
	if err != nil {
		t.Fatalf("ParseConfig failed: %v", err)
	}

	if parsed.DefaultBranch != cfg.DefaultBranch {
		t.Errorf("round-trip DefaultBranch = %q, want %q", parsed.DefaultBranch, cfg.DefaultBranch)
	}
	if len(parsed.DocumentPaths) != len(cfg.DocumentPaths) {
		t.Errorf("round-trip DocumentPaths length = %d, want %d", len(parsed.DocumentPaths), len(cfg.DocumentPaths))
	}
	if len(parsed.FilePatterns) != len(cfg.FilePatterns) {
		t.Errorf("round-trip FilePatterns length = %d, want %d", len(parsed.FilePatterns), len(cfg.FilePatterns))
	}
}

func TestMarshalJSON_Indented(t *testing.T) {
	cfg := DefaultConfig()
	data, err := cfg.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	s := string(data)
	// Indented JSON should contain newlines and leading spaces.
	if len(s) == 0 {
		t.Fatal("MarshalJSON returned empty")
	}
	if s[0] != '{' {
		t.Errorf("expected JSON to start with '{', got %q", s[0:1])
	}
	// Should have indented fields.
	if !json.Valid(data) {
		t.Errorf("MarshalJSON produced invalid JSON")
	}
}

func TestParseConfig_Invalid(t *testing.T) {
	_, err := ParseConfig([]byte("not json"))
	if err == nil {
		t.Error("ParseConfig should fail on invalid JSON")
	}
}

func TestParseConfig_EmptyObject(t *testing.T) {
	cfg, err := ParseConfig([]byte(`{}`))
	if err != nil {
		t.Fatalf("ParseConfig failed: %v", err)
	}
	if cfg.DefaultBranch != "" {
		t.Errorf("DefaultBranch = %q, want empty", cfg.DefaultBranch)
	}
	if cfg.DocumentPaths != nil {
		t.Errorf("DocumentPaths = %v, want nil", cfg.DocumentPaths)
	}
}

func TestParseConfig_CustomValues(t *testing.T) {
	input := `{
		"document_paths": ["custom/"],
		"file_patterns": ["*.txt"],
		"default_branch": "develop"
	}`
	cfg, err := ParseConfig([]byte(input))
	if err != nil {
		t.Fatalf("ParseConfig failed: %v", err)
	}
	if cfg.DefaultBranch != "develop" {
		t.Errorf("DefaultBranch = %q, want %q", cfg.DefaultBranch, "develop")
	}
	if len(cfg.DocumentPaths) != 1 || cfg.DocumentPaths[0] != "custom/" {
		t.Errorf("DocumentPaths = %v, want [custom/]", cfg.DocumentPaths)
	}
	if len(cfg.FilePatterns) != 1 || cfg.FilePatterns[0] != "*.txt" {
		t.Errorf("FilePatterns = %v, want [*.txt]", cfg.FilePatterns)
	}
}
