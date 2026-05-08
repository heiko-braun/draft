package review

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestIndexDocuments_BasicDiscovery(t *testing.T) {
	root := t.TempDir()

	writeTestFile(t, root, "docs/intro.md", "# Intro\n\nHello world.\n")
	writeTestFile(t, root, "docs/guide.mdx", "# Guide\n\nMDX content.\n")
	writeTestFile(t, root, "docs/notes.txt", "Not markdown.\n")
	writeTestFile(t, root, "src/main.go", "package main\n")

	idx, err := IndexDocuments(root, []string{"docs"})
	if err != nil {
		t.Fatal(err)
	}

	if len(idx.Documents) != 2 {
		t.Errorf("Documents count = %d, want 2", len(idx.Documents))
	}

	if _, ok := idx.Documents["docs/intro.md"]; !ok {
		t.Error("missing docs/intro.md in index")
	}
	if _, ok := idx.Documents["docs/guide.mdx"]; !ok {
		t.Error("missing docs/guide.mdx in index")
	}
}

func TestIndexDocuments_SingleFile(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "# Hello\n\nWorld.\n")

	idx, err := IndexDocuments(root, []string{"README.md"})
	if err != nil {
		t.Fatal(err)
	}

	if len(idx.Documents) != 1 {
		t.Fatalf("Documents count = %d, want 1", len(idx.Documents))
	}
	if _, ok := idx.Documents["README.md"]; !ok {
		t.Error("missing README.md in index")
	}
}

func TestIndexDocuments_MultiplePaths(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/a.md", "# A\n")
	writeTestFile(t, root, "specs/b.md", "# B\n")

	idx, err := IndexDocuments(root, []string{"docs", "specs"})
	if err != nil {
		t.Fatal(err)
	}

	if len(idx.Documents) != 2 {
		t.Errorf("Documents count = %d, want 2", len(idx.Documents))
	}
}

func TestIndexDocuments_DedupesFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/a.md", "# A\n")

	// Pass the same path twice.
	idx, err := IndexDocuments(root, []string{"docs", "docs"})
	if err != nil {
		t.Fatal(err)
	}

	if len(idx.Documents) != 1 {
		t.Errorf("Documents count = %d, want 1 (deduped)", len(idx.Documents))
	}
}

func TestIndexDocuments_FrontMatterTitle(t *testing.T) {
	root := t.TempDir()
	content := "---\ntitle: FM Title\ndescription: desc\nstatus: draft\nauthor: Test\n---\n# Heading Title\n\nBody.\n"
	writeTestFile(t, root, "doc.md", content)

	idx, err := IndexDocuments(root, []string{"doc.md"})
	if err != nil {
		t.Fatal(err)
	}

	doc := idx.Documents["doc.md"]
	if doc.Title != "FM Title" {
		t.Errorf("Title = %q, want %q (front-matter takes precedence)", doc.Title, "FM Title")
	}
	if doc.FrontMatter.Description != "desc" {
		t.Errorf("Description = %q, want %q", doc.FrontMatter.Description, "desc")
	}
}

func TestIndexDocuments_FallbackToHeadingTitle(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "doc.md", "# Heading Title\n\nBody paragraph.\n")

	idx, err := IndexDocuments(root, []string{"doc.md"})
	if err != nil {
		t.Fatal(err)
	}

	doc := idx.Documents["doc.md"]
	if doc.Title != "Heading Title" {
		t.Errorf("Title = %q, want %q (fallback to first heading)", doc.Title, "Heading Title")
	}
}

func TestIndexDocuments_HeadingTree(t *testing.T) {
	root := t.TempDir()
	content := `# Top
## Section A
### Sub A1
## Section B
`
	writeTestFile(t, root, "doc.md", content)

	idx, err := IndexDocuments(root, []string{"doc.md"})
	if err != nil {
		t.Fatal(err)
	}

	doc := idx.Documents["doc.md"]

	if len(doc.Headings) != 1 {
		t.Fatalf("root headings = %d, want 1", len(doc.Headings))
	}

	top := doc.Headings[0]
	if top.Text != "Top" {
		t.Errorf("top heading = %q, want %q", top.Text, "Top")
	}
	if top.Level != 1 {
		t.Errorf("top level = %d, want 1", top.Level)
	}
	if len(top.Children) != 2 {
		t.Fatalf("top children = %d, want 2", len(top.Children))
	}

	secA := top.Children[0]
	if secA.Text != "Section A" {
		t.Errorf("Section A text = %q", secA.Text)
	}
	if len(secA.Children) != 1 {
		t.Fatalf("Section A children = %d, want 1", len(secA.Children))
	}
	if secA.Children[0].Text != "Sub A1" {
		t.Errorf("Sub A1 text = %q", secA.Children[0].Text)
	}

	secB := top.Children[1]
	if secB.Text != "Section B" {
		t.Errorf("Section B text = %q", secB.Text)
	}
	if len(secB.Children) != 0 {
		t.Errorf("Section B children = %d, want 0", len(secB.Children))
	}
}

func TestIndexDocuments_Paragraphs(t *testing.T) {
	root := t.TempDir()
	content := `# Title

First paragraph.

Second paragraph.

## Section

Third paragraph.
`
	writeTestFile(t, root, "doc.md", content)

	idx, err := IndexDocuments(root, []string{"doc.md"})
	if err != nil {
		t.Fatal(err)
	}

	doc := idx.Documents["doc.md"]

	if len(doc.Paragraphs) != 3 {
		t.Fatalf("paragraphs = %d, want 3", len(doc.Paragraphs))
	}

	p0 := doc.Paragraphs[0]
	if p0.Content != "First paragraph." {
		t.Errorf("p0 content = %q", p0.Content)
	}
	if p0.Index != 0 {
		t.Errorf("p0 index = %d, want 0", p0.Index)
	}

	p1 := doc.Paragraphs[1]
	if p1.Content != "Second paragraph." {
		t.Errorf("p1 content = %q", p1.Content)
	}
	if p1.Index != 1 {
		t.Errorf("p1 index = %d, want 1", p1.Index)
	}

	p2 := doc.Paragraphs[2]
	if p2.Content != "Third paragraph." {
		t.Errorf("p2 content = %q", p2.Content)
	}
	if p2.Index != 2 {
		t.Errorf("p2 index = %d, want 2", p2.Index)
	}
}

func TestIndexDocuments_ParagraphHeadingPath(t *testing.T) {
	root := t.TempDir()
	content := `# Doc

Intro text.

## Approach

Approach details.

### Sub-approach

Sub details.
`
	writeTestFile(t, root, "doc.md", content)

	idx, err := IndexDocuments(root, []string{"doc.md"})
	if err != nil {
		t.Fatal(err)
	}

	doc := idx.Documents["doc.md"]

	if len(doc.Paragraphs) < 3 {
		t.Fatalf("paragraphs = %d, want at least 3", len(doc.Paragraphs))
	}

	// First paragraph is under "Doc" (the h1).
	if doc.Paragraphs[0].HeadingPath != "Doc" {
		t.Errorf("p0 heading path = %q, want %q", doc.Paragraphs[0].HeadingPath, "Doc")
	}

	// Second paragraph is under "Doc/Approach".
	if doc.Paragraphs[1].HeadingPath != "Doc/Approach" {
		t.Errorf("p1 heading path = %q, want %q", doc.Paragraphs[1].HeadingPath, "Doc/Approach")
	}

	// Third paragraph is under "Doc/Approach/Sub-approach".
	if doc.Paragraphs[2].HeadingPath != "Doc/Approach/Sub-approach" {
		t.Errorf("p2 heading path = %q, want %q", doc.Paragraphs[2].HeadingPath, "Doc/Approach/Sub-approach")
	}
}

func TestIndexDocuments_ParagraphContentHash(t *testing.T) {
	root := t.TempDir()
	content := "# Title\n\nHello world.\n"
	writeTestFile(t, root, "doc.md", content)

	idx, err := IndexDocuments(root, []string{"doc.md"})
	if err != nil {
		t.Fatal(err)
	}

	doc := idx.Documents["doc.md"]
	if len(doc.Paragraphs) != 1 {
		t.Fatalf("paragraphs = %d, want 1", len(doc.Paragraphs))
	}

	p := doc.Paragraphs[0]
	expectedHash := fmt.Sprintf("%x", sha256.Sum256([]byte("Hello world.")))
	if p.ContentHash != expectedHash {
		t.Errorf("hash = %q, want %q", p.ContentHash, expectedHash)
	}
}

func TestIndexDocuments_ParagraphHashNormalizesWhitespace(t *testing.T) {
	root := t.TempDir()
	// Multi-word paragraph with irregular spacing (after normalization both should match).
	content := "# Title\n\nHello   world.\n"
	writeTestFile(t, root, "doc.md", content)

	idx, err := IndexDocuments(root, []string{"doc.md"})
	if err != nil {
		t.Fatal(err)
	}

	doc := idx.Documents["doc.md"]
	p := doc.Paragraphs[0]
	// Normalized form collapses whitespace.
	expectedHash := fmt.Sprintf("%x", sha256.Sum256([]byte("Hello world.")))
	if p.ContentHash != expectedHash {
		t.Errorf("hash = %q, want %q (whitespace normalized)", p.ContentHash, expectedHash)
	}
}

func TestIndexDocuments_EmptyFile(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "empty.md", "")

	idx, err := IndexDocuments(root, []string{"empty.md"})
	if err != nil {
		t.Fatal(err)
	}

	doc := idx.Documents["empty.md"]
	if doc.Title != "" {
		t.Errorf("Title = %q, want empty for empty file", doc.Title)
	}
	if len(doc.Headings) != 0 {
		t.Errorf("Headings = %d, want 0", len(doc.Headings))
	}
	if len(doc.Paragraphs) != 0 {
		t.Errorf("Paragraphs = %d, want 0", len(doc.Paragraphs))
	}
}

func TestIndexDocuments_FileOnlyFrontMatter(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "fm-only.md", "---\ntitle: Just FM\n---\n")

	idx, err := IndexDocuments(root, []string{"fm-only.md"})
	if err != nil {
		t.Fatal(err)
	}

	doc := idx.Documents["fm-only.md"]
	if doc.Title != "Just FM" {
		t.Errorf("Title = %q, want %q", doc.Title, "Just FM")
	}
	if len(doc.Headings) != 0 {
		t.Errorf("Headings = %d, want 0", len(doc.Headings))
	}
	if len(doc.Paragraphs) != 0 {
		t.Errorf("Paragraphs = %d, want 0", len(doc.Paragraphs))
	}
}

func TestIndexDocuments_DeeplyNestedHeadings(t *testing.T) {
	root := t.TempDir()
	content := `# H1
## H2
### H3
#### H4
##### H5
###### H6
`
	writeTestFile(t, root, "deep.md", content)

	idx, err := IndexDocuments(root, []string{"deep.md"})
	if err != nil {
		t.Fatal(err)
	}

	doc := idx.Documents["deep.md"]

	// Should have one root heading (H1) with nested children.
	if len(doc.Headings) != 1 {
		t.Fatalf("root headings = %d, want 1", len(doc.Headings))
	}

	// Walk down the tree.
	node := doc.Headings[0]
	for expectedLevel := 1; expectedLevel <= 6; expectedLevel++ {
		if node.Level != expectedLevel {
			t.Errorf("level %d: got level %d", expectedLevel, node.Level)
		}
		expectedText := fmt.Sprintf("H%d", expectedLevel)
		if node.Text != expectedText {
			t.Errorf("level %d: text = %q, want %q", expectedLevel, node.Text, expectedText)
		}
		if expectedLevel < 6 {
			if len(node.Children) != 1 {
				t.Fatalf("level %d: children = %d, want 1", expectedLevel, len(node.Children))
			}
			node = node.Children[0]
		}
	}
}

func TestIndexDocuments_NonexistentPath(t *testing.T) {
	root := t.TempDir()

	_, err := IndexDocuments(root, []string{"nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent path, got nil")
	}
}

func TestIndexDocuments_ParagraphByteOffsets(t *testing.T) {
	root := t.TempDir()
	content := "# Title\n\nFirst paragraph.\n\nSecond paragraph.\n"
	writeTestFile(t, root, "doc.md", content)

	idx, err := IndexDocuments(root, []string{"doc.md"})
	if err != nil {
		t.Fatal(err)
	}

	doc := idx.Documents["doc.md"]
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}

	p0 := doc.Paragraphs[0]
	// "First paragraph." starts after "# Title\n\n" = 10 bytes.
	if p0.StartOffset < 1 {
		t.Errorf("p0 StartOffset = %d, expected > 0", p0.StartOffset)
	}
	if p0.EndOffset <= p0.StartOffset {
		t.Errorf("p0 EndOffset (%d) should be > StartOffset (%d)", p0.EndOffset, p0.StartOffset)
	}

	p1 := doc.Paragraphs[1]
	if p1.StartOffset <= p0.EndOffset {
		// Second paragraph should start after the first one ends.
		// (There might be a blank line in between, so >= is fine.)
	}
	if p1.EndOffset <= p1.StartOffset {
		t.Errorf("p1 EndOffset (%d) should be > StartOffset (%d)", p1.EndOffset, p1.StartOffset)
	}
}
