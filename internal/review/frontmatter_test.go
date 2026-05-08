package review

import (
	"testing"
)

func TestExtractFrontMatter_ValidYAML(t *testing.T) {
	input := []byte(`---
title: My Document
description: A test document
status: proposed
author: Jane Doe
---
# Body starts here
`)

	fm, body := ExtractFrontMatter(input)

	if fm.Title != "My Document" {
		t.Errorf("Title = %q, want %q", fm.Title, "My Document")
	}
	if fm.Description != "A test document" {
		t.Errorf("Description = %q, want %q", fm.Description, "A test document")
	}
	if fm.Status != "proposed" {
		t.Errorf("Status = %q, want %q", fm.Status, "proposed")
	}
	if fm.Author != "Jane Doe" {
		t.Errorf("Author = %q, want %q", fm.Author, "Jane Doe")
	}

	want := "# Body starts here\n"
	if string(body) != want {
		t.Errorf("body = %q, want %q", string(body), want)
	}
}

func TestExtractFrontMatter_NoFrontMatter(t *testing.T) {
	input := []byte("# Just a heading\n\nSome text.\n")

	fm, body := ExtractFrontMatter(input)

	if fm.Title != "" {
		t.Errorf("Title = %q, want empty", fm.Title)
	}
	if string(body) != string(input) {
		t.Errorf("body should equal input when no front-matter present")
	}
}

func TestExtractFrontMatter_EmptyFile(t *testing.T) {
	input := []byte("")

	fm, body := ExtractFrontMatter(input)

	if fm.Title != "" {
		t.Errorf("Title = %q, want empty", fm.Title)
	}
	if len(body) != 0 {
		t.Errorf("body should be empty, got %q", string(body))
	}
}

func TestExtractFrontMatter_OnlyFrontMatter(t *testing.T) {
	input := []byte("---\ntitle: Only FM\n---\n")

	fm, body := ExtractFrontMatter(input)

	if fm.Title != "Only FM" {
		t.Errorf("Title = %q, want %q", fm.Title, "Only FM")
	}
	if len(body) != 0 {
		t.Errorf("body should be empty, got %q", string(body))
	}
}

func TestExtractFrontMatter_PartialFields(t *testing.T) {
	input := []byte("---\ntitle: Partial\n---\nContent here.\n")

	fm, body := ExtractFrontMatter(input)

	if fm.Title != "Partial" {
		t.Errorf("Title = %q, want %q", fm.Title, "Partial")
	}
	if fm.Description != "" {
		t.Errorf("Description = %q, want empty", fm.Description)
	}
	if fm.Status != "" {
		t.Errorf("Status = %q, want empty", fm.Status)
	}

	want := "Content here.\n"
	if string(body) != want {
		t.Errorf("body = %q, want %q", string(body), want)
	}
}

func TestExtractFrontMatter_InvalidYAML(t *testing.T) {
	input := []byte("---\n: invalid: yaml: [broken\n---\nBody.\n")

	fm, body := ExtractFrontMatter(input)

	// Invalid YAML should return empty front-matter and original content.
	if fm.Title != "" {
		t.Errorf("Title = %q, want empty on invalid YAML", fm.Title)
	}
	if string(body) != string(input) {
		t.Errorf("body should equal original input on invalid YAML")
	}
}

func TestExtractFrontMatter_MissingClosingDelimiter(t *testing.T) {
	input := []byte("---\ntitle: Unclosed\nSome text without closing delimiter.\n")

	fm, body := ExtractFrontMatter(input)

	if fm.Title != "" {
		t.Errorf("Title = %q, want empty when closing delimiter missing", fm.Title)
	}
	if string(body) != string(input) {
		t.Errorf("body should equal original input when closing delimiter missing")
	}
}
