package search

import (
	"path/filepath"
	"testing"
)

func indexedStore(t *testing.T, files map[string]string) *Store {
	t.Helper()
	root := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	for rel, content := range files {
		writeFile(t, root, rel, content)
	}

	s, err := OpenStore(dbPath, root)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Index(s, root, false); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { s.Close() })
	return s
}

func TestSearch_FTSRanking(t *testing.T) {
	s := indexedStore(t, map[string]string{
		"auth.go":    "package auth\n// authentication middleware handles token validation\nfunc AuthMiddleware() {}\n",
		"readme.md":  "# Project\nThis project has nothing to do with authentication.\n",
		"handler.go": "package handler\n// generic handler for HTTP requests\nfunc Handle() {}\n",
	})

	results, err := Search(s, "authentication middleware", 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) == 0 {
		t.Fatal("expected results")
	}

	// auth.go should rank first (has both terms).
	if results[0].Path != "auth.go" {
		t.Errorf("expected auth.go first, got %s", results[0].Path)
	}
}

func TestSearch_PathWeighting(t *testing.T) {
	s := indexedStore(t, map[string]string{
		"auth_middleware.go": "package auth\nfunc Run() {}\n",
		"utils.go":           "package utils\n// auth_middleware is referenced here\nfunc Helper() {}\n",
	})

	results, err := Search(s, "auth middleware", 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) == 0 {
		t.Fatal("expected results")
	}

	// File with matching path should rank higher.
	if results[0].Path != "auth_middleware.go" {
		t.Errorf("expected auth_middleware.go first, got %s", results[0].Path)
	}
}

func TestSearch_TrigramSubstring(t *testing.T) {
	s := indexedStore(t, map[string]string{
		"config.go":  "package config\ntype AppCfgLoader struct{}\n",
		"service.go": "package service\ntype UserConfigService struct{}\n",
	})

	results, err := Search(s, "CfgLoad", 10)
	if err != nil {
		t.Fatal(err)
	}

	// Should find config.go (contains AppCfgLoader).
	found := false
	for _, r := range results {
		if r.Path == "config.go" {
			found = true
		}
	}
	if !found {
		t.Error("expected config.go in results for CfgLoad")
	}
}

func TestSearch_TrigramMinLength(t *testing.T) {
	s := indexedStore(t, map[string]string{
		"a.go": "package a\nfunc AB() {}\n",
	})

	// 2-char query should not crash.
	results, err := Search(s, "AB", 10)
	if err != nil {
		t.Fatalf("expected no error for short query, got: %v", err)
	}

	// May return nil or empty — that's fine.
	_ = results
}

func TestSearch_QueryRouting(t *testing.T) {
	tests := []struct {
		query    string
		expected QueryType
	}{
		{"error handling", QueryNaturalLanguage},
		{"CfgLoader", QuerySubstring},
		{"snake_case", QuerySubstring},
		{"AuthHandler middleware", QueryMixed},
		{"search", QueryMixed}, // single word → mixed
	}

	for _, tt := range tests {
		got := ClassifyQuery(tt.query)
		if got != tt.expected {
			t.Errorf("ClassifyQuery(%q) = %d, want %d", tt.query, got, tt.expected)
		}
	}
}

func TestSearch_ScoreMerging(t *testing.T) {
	fts := []SearchResult{
		{Path: "a.go", Score: 10},
		{Path: "b.go", Score: 5},
	}
	tri := []SearchResult{
		{Path: "b.go", Score: 10},
		{Path: "c.go", Score: 5},
	}

	merged := mergeResults(fts, tri, 10)

	// All three files should be present.
	if len(merged) != 3 {
		t.Fatalf("expected 3 merged results, got %d", len(merged))
	}

	// b.go should have contributions from both backends.
	var bScore float64
	for _, r := range merged {
		if r.Path == "b.go" {
			bScore = r.Score
		}
	}
	if bScore == 0 {
		t.Error("b.go should have a non-zero merged score")
	}
}

func TestSearch_SnippetFromFTS(t *testing.T) {
	s := indexedStore(t, map[string]string{
		"main.go": "package main\nimport \"fmt\"\nfunc main() {\n\tfmt.Println(\"hello world\")\n}\n",
	})

	results, err := Search(s, "hello world", 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) == 0 {
		t.Fatal("expected results")
	}

	// Snippet should contain the markers.
	if results[0].Snippet == "" {
		t.Error("expected non-empty snippet from FTS")
	}
}

func TestSearch_Limit(t *testing.T) {
	files := map[string]string{}
	for i := 0; i < 10; i++ {
		name := filepath.Join("pkg", string(rune('a'+i))+".go")
		files[name] = "package pkg\nfunc Handler() {}\n"
	}

	s := indexedStore(t, files)

	results, err := Search(s, "Handler", 3)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) > 3 {
		t.Errorf("expected at most 3 results, got %d", len(results))
	}
}

func TestFormatResults_Empty(t *testing.T) {
	out := FormatResults(nil)
	if out != "No results found.\n" {
		t.Errorf("unexpected output for empty results: %q", out)
	}
}

func TestFormatResults_WithResults(t *testing.T) {
	results := []SearchResult{
		{Path: "a.go", Score: 0.87, Snippet: "…the »test« content…"},
	}
	out := FormatResults(results)
	if out == "" {
		t.Error("expected non-empty formatted output")
	}
}
