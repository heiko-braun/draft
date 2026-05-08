package review

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

// testHash computes the same SHA-256 hex digest used by the indexer.
func testHash(s string) string {
	normalized := strings.Join(strings.Fields(s), " ")
	h := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("%x", h)
}

// buildTestDoc creates a Document with the given paragraphs for testing.
func buildTestDoc(paragraphs []*Paragraph) *Document {
	return &Document{
		RelPath:    "test.md",
		Paragraphs: paragraphs,
	}
}

// buildTestIndex creates a DocIndex containing a single document.
func buildTestIndex(relPath string, doc *Document) *DocIndex {
	return &DocIndex{
		Documents: map[string]*Document{
			relPath: doc,
		},
	}
}

func TestResolveAnchors_ExactMatch(t *testing.T) {
	content := "This is the original paragraph content."
	hash := testHash(content)

	doc := buildTestDoc([]*Paragraph{
		{Content: content, HeadingPath: "Goal", Index: 0, ContentHash: hash},
	})
	idx := buildTestIndex("doc.md", doc)

	threads := []Thread{
		{
			ID:       "thread-1",
			Document: "doc.md",
			Anchor: Anchor{
				HeadingPath:    []string{"Goal"},
				ParagraphIndex: 0,
				Excerpt:        "original paragraph",
				ContentHash:    hash,
			},
		},
	}

	results := ResolveAnchors(idx, threads)

	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}

	r := results[0]
	if r.ThreadID != "thread-1" {
		t.Errorf("ThreadID = %q, want %q", r.ThreadID, "thread-1")
	}
	if r.Status != AnchorExact {
		t.Errorf("Status = %v, want AnchorExact", r.Status)
	}
}

func TestResolveAnchors_ExactMatch_NestedHeadingPath(t *testing.T) {
	content := "Deeply nested paragraph."
	hash := testHash(content)

	doc := buildTestDoc([]*Paragraph{
		{Content: content, HeadingPath: "Doc/Approach/Details", Index: 2, ContentHash: hash},
	})
	idx := buildTestIndex("doc.md", doc)

	threads := []Thread{
		{
			ID:       "thread-2",
			Document: "doc.md",
			Anchor: Anchor{
				HeadingPath:    []string{"Doc", "Approach", "Details"},
				ParagraphIndex: 2,
				ContentHash:    hash,
			},
		},
	}

	results := ResolveAnchors(idx, threads)

	if results[0].Status != AnchorExact {
		t.Errorf("Status = %v, want AnchorExact", results[0].Status)
	}
}

func TestResolveAnchors_StructuralMatch_IndexShifted(t *testing.T) {
	// The paragraph has the same content but its index shifted by 1
	// (e.g. a new paragraph was inserted above it).
	content := "This paragraph moved down by one position."
	hash := testHash(content)

	doc := buildTestDoc([]*Paragraph{
		{Content: "New paragraph inserted above.", HeadingPath: "Goal", Index: 0, ContentHash: testHash("New paragraph inserted above.")},
		{Content: content, HeadingPath: "Goal", Index: 1, ContentHash: hash},
	})
	idx := buildTestIndex("doc.md", doc)

	threads := []Thread{
		{
			ID:       "thread-3",
			Document: "doc.md",
			Anchor: Anchor{
				HeadingPath:    []string{"Goal"},
				ParagraphIndex: 0, // Was at index 0, now at index 1.
				Excerpt:        "paragraph moved down",
				ContentHash:    "stale-hash-that-no-longer-matches",
			},
		},
	}

	results := ResolveAnchors(idx, threads)

	r := results[0]
	if r.Status != AnchorStructural {
		t.Errorf("Status = %v, want AnchorStructural", r.Status)
	}
	if r.Anchor.ParagraphIndex != 1 {
		t.Errorf("ParagraphIndex = %d, want 1 (re-anchored)", r.Anchor.ParagraphIndex)
	}
	if r.Anchor.ContentHash != hash {
		t.Errorf("ContentHash not updated to new paragraph hash")
	}
}

func TestResolveAnchors_StructuralMatch_WithinWindow(t *testing.T) {
	// Paragraph shifted by exactly 3 positions (within ±3 window).
	content := "A paragraph that shifted by three positions."

	doc := buildTestDoc([]*Paragraph{
		{Content: "filler 0", HeadingPath: "Section", Index: 0, ContentHash: testHash("filler 0")},
		{Content: "filler 1", HeadingPath: "Section", Index: 1, ContentHash: testHash("filler 1")},
		{Content: "filler 2", HeadingPath: "Section", Index: 2, ContentHash: testHash("filler 2")},
		{Content: content, HeadingPath: "Section", Index: 3, ContentHash: testHash(content)},
	})
	idx := buildTestIndex("doc.md", doc)

	threads := []Thread{
		{
			ID:       "thread-window",
			Document: "doc.md",
			Anchor: Anchor{
				HeadingPath:    []string{"Section"},
				ParagraphIndex: 0, // Original index was 0, now at 3 (shift of 3).
				Excerpt:        "shifted by three",
				ContentHash:    "old-hash",
			},
		},
	}

	results := ResolveAnchors(idx, threads)

	if results[0].Status != AnchorStructural {
		t.Errorf("Status = %v, want AnchorStructural (within ±3 window)", results[0].Status)
	}
}

func TestResolveAnchors_StructuralMatch_OutsideWindow(t *testing.T) {
	// Paragraph shifted by 4 positions (outside ±3 window).
	// Should NOT get structural match, but may get fuzzy match.
	content := "This paragraph moved far away from its original position in this document section."

	doc := buildTestDoc([]*Paragraph{
		{Content: "filler 0", HeadingPath: "Section", Index: 0, ContentHash: testHash("filler 0")},
		{Content: "filler 1", HeadingPath: "Section", Index: 1, ContentHash: testHash("filler 1")},
		{Content: "filler 2", HeadingPath: "Section", Index: 2, ContentHash: testHash("filler 2")},
		{Content: "filler 3", HeadingPath: "Section", Index: 3, ContentHash: testHash("filler 3")},
		{Content: content, HeadingPath: "Section", Index: 4, ContentHash: testHash(content)},
	})
	idx := buildTestIndex("doc.md", doc)

	threads := []Thread{
		{
			ID:       "thread-far",
			Document: "doc.md",
			Anchor: Anchor{
				HeadingPath:    []string{"Section"},
				ParagraphIndex: 0, // Original index was 0, now at 4 (shift of 4).
				Excerpt:        "paragraph moved far away",
				ContentHash:    "old-hash",
			},
		},
	}

	results := ResolveAnchors(idx, threads)

	// It should fall through to fuzzy (not structural) because index diff is 4.
	if results[0].Status == AnchorStructural {
		t.Errorf("Status = AnchorStructural, expected fuzzy or orphaned (outside ±3 window)")
	}
	// It should find via fuzzy since the excerpt is present.
	if results[0].Status != AnchorFuzzy {
		t.Errorf("Status = %v, want AnchorFuzzy", results[0].Status)
	}
}

func TestResolveAnchors_FuzzyMatch_SectionRenamed(t *testing.T) {
	// The section was renamed, so heading_path no longer matches,
	// but the paragraph content is still present elsewhere.
	content := "This content was moved to a completely different section in the document."

	doc := buildTestDoc([]*Paragraph{
		{Content: content, HeadingPath: "NewSection/Details", Index: 0, ContentHash: testHash(content)},
	})
	idx := buildTestIndex("doc.md", doc)

	threads := []Thread{
		{
			ID:       "thread-4",
			Document: "doc.md",
			Anchor: Anchor{
				HeadingPath:    []string{"OldSection", "Details"},
				ParagraphIndex: 5,
				Excerpt:        "moved to a completely different section",
				ContentHash:    "stale-hash",
			},
		},
	}

	results := ResolveAnchors(idx, threads)

	r := results[0]
	if r.Status != AnchorFuzzy {
		t.Errorf("Status = %v, want AnchorFuzzy", r.Status)
	}
	// Anchor should be re-anchored to the new location.
	if r.Anchor.ParagraphIndex != 0 {
		t.Errorf("ParagraphIndex = %d, want 0", r.Anchor.ParagraphIndex)
	}
	expectedPath := []string{"NewSection", "Details"}
	if len(r.Anchor.HeadingPath) != len(expectedPath) {
		t.Fatalf("HeadingPath = %v, want %v", r.Anchor.HeadingPath, expectedPath)
	}
	for i, part := range expectedPath {
		if r.Anchor.HeadingPath[i] != part {
			t.Errorf("HeadingPath[%d] = %q, want %q", i, r.Anchor.HeadingPath[i], part)
		}
	}
}

func TestResolveAnchors_FuzzyMatch_WhitespaceInsensitive(t *testing.T) {
	// The paragraph has different whitespace than the excerpt.
	content := "This   paragraph   has   extra   spaces   but   should   still   match."

	doc := buildTestDoc([]*Paragraph{
		{Content: content, HeadingPath: "Section", Index: 0, ContentHash: testHash(content)},
	})
	idx := buildTestIndex("doc.md", doc)

	threads := []Thread{
		{
			ID:       "thread-ws",
			Document: "doc.md",
			Anchor: Anchor{
				HeadingPath:    []string{"OtherSection"},
				ParagraphIndex: 2,
				Excerpt:        "paragraph has extra spaces but should still match",
				ContentHash:    "stale-hash",
			},
		},
	}

	results := ResolveAnchors(idx, threads)

	if results[0].Status != AnchorFuzzy {
		t.Errorf("Status = %v, want AnchorFuzzy (whitespace-insensitive match)", results[0].Status)
	}
}

func TestResolveAnchors_FuzzyMatch_CaseInsensitive(t *testing.T) {
	content := "The Resolution Cascade Handles Case Differences gracefully."

	doc := buildTestDoc([]*Paragraph{
		{Content: content, HeadingPath: "Approach", Index: 0, ContentHash: testHash(content)},
	})
	idx := buildTestIndex("doc.md", doc)

	threads := []Thread{
		{
			ID:       "thread-case",
			Document: "doc.md",
			Anchor: Anchor{
				HeadingPath:    []string{"Other"},
				ParagraphIndex: 0,
				Excerpt:        "resolution cascade handles case differences",
				ContentHash:    "stale-hash",
			},
		},
	}

	results := ResolveAnchors(idx, threads)

	if results[0].Status != AnchorFuzzy {
		t.Errorf("Status = %v, want AnchorFuzzy (case-insensitive match)", results[0].Status)
	}
}

func TestResolveAnchors_Orphaned_ParagraphDeleted(t *testing.T) {
	// Document exists but the paragraph is gone entirely.
	doc := buildTestDoc([]*Paragraph{
		{Content: "Some other content.", HeadingPath: "Intro", Index: 0, ContentHash: testHash("Some other content.")},
	})
	idx := buildTestIndex("doc.md", doc)

	threads := []Thread{
		{
			ID:       "thread-5",
			Document: "doc.md",
			Anchor: Anchor{
				HeadingPath:    []string{"Intro"},
				ParagraphIndex: 0,
				Excerpt:        "completely unique text that does not exist anywhere",
				ContentHash:    "stale-hash",
			},
		},
	}

	results := ResolveAnchors(idx, threads)

	r := results[0]
	if r.Status != AnchorOrphaned {
		t.Errorf("Status = %v, want AnchorOrphaned", r.Status)
	}
	// Original anchor must be preserved.
	if r.Anchor.ContentHash != "stale-hash" {
		t.Errorf("original anchor not preserved: ContentHash = %q", r.Anchor.ContentHash)
	}
}

func TestResolveAnchors_Orphaned_DocumentMissing(t *testing.T) {
	// Thread references a document that does not exist in the index.
	idx := &DocIndex{Documents: map[string]*Document{}}

	threads := []Thread{
		{
			ID:       "thread-6",
			Document: "nonexistent.md",
			Anchor: Anchor{
				HeadingPath:    []string{"Whatever"},
				ParagraphIndex: 0,
				Excerpt:        "anything",
				ContentHash:    "hash",
			},
		},
	}

	results := ResolveAnchors(idx, threads)

	if results[0].Status != AnchorOrphaned {
		t.Errorf("Status = %v, want AnchorOrphaned (missing document)", results[0].Status)
	}
}

func TestResolveAnchors_Orphaned_EmptyExcerpt(t *testing.T) {
	// If excerpt is empty and hash doesn't match, structural and fuzzy cannot
	// work, so the thread should be orphaned.
	doc := buildTestDoc([]*Paragraph{
		{Content: "Some content.", HeadingPath: "Section", Index: 0, ContentHash: testHash("Some content.")},
	})
	idx := buildTestIndex("doc.md", doc)

	threads := []Thread{
		{
			ID:       "thread-noe",
			Document: "doc.md",
			Anchor: Anchor{
				HeadingPath:    []string{"Section"},
				ParagraphIndex: 0,
				Excerpt:        "",
				ContentHash:    "different-hash",
			},
		},
	}

	results := ResolveAnchors(idx, threads)

	if results[0].Status != AnchorOrphaned {
		t.Errorf("Status = %v, want AnchorOrphaned (empty excerpt, hash mismatch)", results[0].Status)
	}
}

func TestResolveAnchors_FuzzyMatch_MinRatioEnforced(t *testing.T) {
	// A very short excerpt against a very long paragraph should fail the
	// minimum match ratio check and be orphaned.
	longContent := strings.Repeat("This is a long paragraph with lots of content. ", 50)

	doc := buildTestDoc([]*Paragraph{
		{Content: longContent, HeadingPath: "Section", Index: 0, ContentHash: testHash(longContent)},
	})
	idx := buildTestIndex("doc.md", doc)

	threads := []Thread{
		{
			ID:       "thread-ratio",
			Document: "doc.md",
			Anchor: Anchor{
				HeadingPath:    []string{"Other"},
				ParagraphIndex: 0,
				Excerpt:        "long",
				ContentHash:    "stale-hash",
			},
		},
	}

	results := ResolveAnchors(idx, threads)

	if results[0].Status != AnchorOrphaned {
		t.Errorf("Status = %v, want AnchorOrphaned (excerpt too short relative to paragraph)", results[0].Status)
	}
}

func TestResolveAnchors_MultipleThreads(t *testing.T) {
	content1 := "First paragraph that will match exactly."
	content2 := "Second paragraph that will be found via fuzzy search in a different location."
	hash1 := testHash(content1)

	doc := buildTestDoc([]*Paragraph{
		{Content: content1, HeadingPath: "Intro", Index: 0, ContentHash: hash1},
		{Content: content2, HeadingPath: "Moved", Index: 1, ContentHash: testHash(content2)},
	})
	idx := buildTestIndex("doc.md", doc)

	threads := []Thread{
		{
			ID:       "t-exact",
			Document: "doc.md",
			Anchor: Anchor{
				HeadingPath:    []string{"Intro"},
				ParagraphIndex: 0,
				ContentHash:    hash1,
				Excerpt:        "match exactly",
			},
		},
		{
			ID:       "t-fuzzy",
			Document: "doc.md",
			Anchor: Anchor{
				HeadingPath:    []string{"Original"},
				ParagraphIndex: 5,
				ContentHash:    "old-hash",
				Excerpt:        "found via fuzzy search in a different location",
			},
		},
		{
			ID:       "t-orphan",
			Document: "doc.md",
			Anchor: Anchor{
				HeadingPath:    []string{"Gone"},
				ParagraphIndex: 99,
				ContentHash:    "missing-hash",
				Excerpt:        "this text no longer exists anywhere in the document at all whatsoever",
			},
		},
	}

	results := ResolveAnchors(idx, threads)

	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}

	statuses := map[string]AnchorStatus{}
	for _, r := range results {
		statuses[r.ThreadID] = r.Status
	}

	if statuses["t-exact"] != AnchorExact {
		t.Errorf("t-exact: %v, want AnchorExact", statuses["t-exact"])
	}
	if statuses["t-fuzzy"] != AnchorFuzzy {
		t.Errorf("t-fuzzy: %v, want AnchorFuzzy", statuses["t-fuzzy"])
	}
	if statuses["t-orphan"] != AnchorOrphaned {
		t.Errorf("t-orphan: %v, want AnchorOrphaned", statuses["t-orphan"])
	}
}

func TestResolveAnchors_PreservesOriginalAnchorOnOrphan(t *testing.T) {
	doc := buildTestDoc([]*Paragraph{
		{Content: "Unrelated content.", HeadingPath: "Section", Index: 0, ContentHash: testHash("Unrelated content.")},
	})
	idx := buildTestIndex("doc.md", doc)

	originalAnchor := Anchor{
		HeadingPath:    []string{"OriginalSection", "SubSection"},
		ParagraphIndex: 7,
		Excerpt:        "this text is gone forever from this document entirely",
		ContentHash:    "original-hash-value",
		CharRange:      [2]int{10, 50},
		SourceRef:      "doc.md:42",
	}

	threads := []Thread{
		{ID: "t-preserve", Document: "doc.md", Anchor: originalAnchor},
	}

	results := ResolveAnchors(idx, threads)

	r := results[0]
	if r.Status != AnchorOrphaned {
		t.Fatalf("Status = %v, want AnchorOrphaned", r.Status)
	}
	if r.Anchor.ContentHash != "original-hash-value" {
		t.Errorf("ContentHash = %q, want preserved original", r.Anchor.ContentHash)
	}
	if r.Anchor.CharRange != [2]int{10, 50} {
		t.Errorf("CharRange = %v, want preserved [10, 50]", r.Anchor.CharRange)
	}
	if r.Anchor.SourceRef != "doc.md:42" {
		t.Errorf("SourceRef = %q, want preserved", r.Anchor.SourceRef)
	}
}

func TestResolveAnchors_ReanchorUpdatesFields(t *testing.T) {
	content := "This paragraph has been moved to a new section and its anchor should be updated accordingly."

	doc := buildTestDoc([]*Paragraph{
		{Content: content, HeadingPath: "NewPlace/Sub", Index: 3, ContentHash: testHash(content)},
	})
	idx := buildTestIndex("doc.md", doc)

	threads := []Thread{
		{
			ID:       "t-reanchor",
			Document: "doc.md",
			Anchor: Anchor{
				HeadingPath:    []string{"OldPlace"},
				ParagraphIndex: 0,
				Excerpt:        "moved to a new section and its anchor should be updated",
				ContentHash:    "old-hash",
				CharRange:      [2]int{5, 60},
				SourceRef:      "doc.md:10",
			},
		},
	}

	results := ResolveAnchors(idx, threads)

	r := results[0]
	if r.Status != AnchorFuzzy {
		t.Fatalf("Status = %v, want AnchorFuzzy", r.Status)
	}
	// HeadingPath should be updated.
	expected := []string{"NewPlace", "Sub"}
	if len(r.Anchor.HeadingPath) != 2 || r.Anchor.HeadingPath[0] != "NewPlace" || r.Anchor.HeadingPath[1] != "Sub" {
		t.Errorf("HeadingPath = %v, want %v", r.Anchor.HeadingPath, expected)
	}
	// ParagraphIndex should be updated.
	if r.Anchor.ParagraphIndex != 3 {
		t.Errorf("ParagraphIndex = %d, want 3", r.Anchor.ParagraphIndex)
	}
	// ContentHash should be updated to the new paragraph's hash.
	if r.Anchor.ContentHash != testHash(content) {
		t.Errorf("ContentHash not updated to new paragraph's hash")
	}
	// CharRange and SourceRef should be preserved from original.
	if r.Anchor.CharRange != [2]int{5, 60} {
		t.Errorf("CharRange = %v, want [5, 60] (preserved)", r.Anchor.CharRange)
	}
	if r.Anchor.SourceRef != "doc.md:10" {
		t.Errorf("SourceRef = %q, want preserved", r.Anchor.SourceRef)
	}
}

func TestResolveAnchors_CascadeOrder_ExactBeforeStructural(t *testing.T) {
	// A paragraph matches both exact (hash at same location) and could
	// also match structurally. Exact should win.
	content := "Paragraph that matches exactly."
	hash := testHash(content)

	doc := buildTestDoc([]*Paragraph{
		{Content: content, HeadingPath: "Section", Index: 0, ContentHash: hash},
	})
	idx := buildTestIndex("doc.md", doc)

	threads := []Thread{
		{
			ID:       "t-cascade",
			Document: "doc.md",
			Anchor: Anchor{
				HeadingPath:    []string{"Section"},
				ParagraphIndex: 0,
				Excerpt:        "matches exactly",
				ContentHash:    hash,
			},
		},
	}

	results := ResolveAnchors(idx, threads)

	if results[0].Status != AnchorExact {
		t.Errorf("Status = %v, want AnchorExact (should win over structural)", results[0].Status)
	}
}

func TestResolveAnchors_CascadeOrder_StructuralBeforeFuzzy(t *testing.T) {
	// The paragraph is at a nearby index under the same heading path.
	// It should match structurally, not fall through to fuzzy.
	content := "A paragraph that appears both under the correct heading and could match anywhere."

	doc := buildTestDoc([]*Paragraph{
		{Content: "filler paragraph at index 0", HeadingPath: "Target", Index: 0, ContentHash: testHash("filler paragraph at index 0")},
		{Content: content, HeadingPath: "Target", Index: 1, ContentHash: testHash(content)},
	})
	idx := buildTestIndex("doc.md", doc)

	threads := []Thread{
		{
			ID:       "t-struct-first",
			Document: "doc.md",
			Anchor: Anchor{
				HeadingPath:    []string{"Target"},
				ParagraphIndex: 0, // Shifted by 1.
				Excerpt:        "appears both under the correct heading",
				ContentHash:    "old-hash",
			},
		},
	}

	results := ResolveAnchors(idx, threads)

	if results[0].Status != AnchorStructural {
		t.Errorf("Status = %v, want AnchorStructural (should win over fuzzy)", results[0].Status)
	}
}

func TestAnchorStatus_String(t *testing.T) {
	tests := []struct {
		status AnchorStatus
		want   string
	}{
		{AnchorExact, "exact"},
		{AnchorStructural, "structural"},
		{AnchorFuzzy, "fuzzy"},
		{AnchorOrphaned, "orphaned"},
		{AnchorStatus(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("AnchorStatus(%d).String() = %q, want %q", int(tt.status), got, tt.want)
		}
	}
}

func TestResolveAnchors_EmptyThreads(t *testing.T) {
	idx := &DocIndex{Documents: map[string]*Document{}}
	results := ResolveAnchors(idx, nil)

	if len(results) != 0 {
		t.Errorf("results = %d, want 0 for nil threads", len(results))
	}

	results = ResolveAnchors(idx, []Thread{})
	if len(results) != 0 {
		t.Errorf("results = %d, want 0 for empty threads", len(results))
	}
}

func TestResolveAnchors_IntegrationWithIndexer(t *testing.T) {
	// Use the real indexer to parse a document, then resolve anchors against it.
	root := t.TempDir()
	content := `# Feature

## Goal

Provide robust anchoring for review threads.

## Approach

The resolution cascade runs exact, structural, then fuzzy.

### Details

Implementation uses normalized substring matching.
`
	writeTestFile(t, root, "feature.md", content)

	idx, err := IndexDocuments(root, []string{"feature.md"})
	if err != nil {
		t.Fatal(err)
	}

	doc := idx.Documents["feature.md"]
	if len(doc.Paragraphs) < 3 {
		t.Fatalf("expected at least 3 paragraphs, got %d", len(doc.Paragraphs))
	}

	// Create a thread anchored to the "Goal" paragraph.
	goalPara := doc.Paragraphs[0]
	threads := []Thread{
		{
			ID:       "t-integration",
			Document: "feature.md",
			Anchor: Anchor{
				HeadingPath:    strings.Split(goalPara.HeadingPath, "/"),
				ParagraphIndex: goalPara.Index,
				Excerpt:        "robust anchoring",
				ContentHash:    goalPara.ContentHash,
			},
		},
	}

	results := ResolveAnchors(idx, threads)

	if results[0].Status != AnchorExact {
		t.Errorf("Status = %v, want AnchorExact (integration test)", results[0].Status)
	}
}
