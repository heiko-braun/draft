package review

import (
	"strings"
)

// structuralWindow is the maximum distance (in paragraph indices) from the
// anchor's recorded paragraph_index that a structural match will consider.
const structuralWindow = 3

// minFuzzyRatio is the minimum ratio of excerpt length to paragraph content
// length required for a fuzzy match to be accepted. This prevents short
// excerpts from matching long paragraphs as false positives.
const minFuzzyRatio = 0.1

// ResolveAnchors runs the resolution cascade for every thread and returns
// one AnchorResult per thread. The cascade order is:
//
//  1. Exact match   — content_hash matches the paragraph at the same heading_path and paragraph_index.
//  2. Structural    — heading_path exists, paragraph at/near index (±3) contains the excerpt.
//  3. Fuzzy search  — search all paragraphs in the document for the excerpt.
//  4. Orphaned      — no match found; original anchor preserved.
func ResolveAnchors(index *DocIndex, threads []Thread) []AnchorResult {
	results := make([]AnchorResult, 0, len(threads))

	for _, t := range threads {
		result := resolveOne(index, t)
		results = append(results, result)
	}

	return results
}

// resolveOne runs the cascade for a single thread.
func resolveOne(index *DocIndex, t Thread) AnchorResult {
	doc, ok := index.Documents[t.Document]
	if !ok {
		return AnchorResult{
			ThreadID: t.ID,
			Status:   AnchorOrphaned,
			Anchor:   t.Anchor,
		}
	}

	// 1. Exact match.
	if result, ok := tryExactMatch(doc, t); ok {
		return result
	}

	// 2. Structural match.
	if result, ok := tryStructuralMatch(doc, t); ok {
		return result
	}

	// 3. Fuzzy search.
	if result, ok := tryFuzzyMatch(doc, t); ok {
		return result
	}

	// 4. Orphaned.
	return AnchorResult{
		ThreadID: t.ID,
		Status:   AnchorOrphaned,
		Anchor:   t.Anchor,
	}
}

// tryExactMatch checks whether the paragraph at the anchor's heading_path
// and paragraph_index still has the same content_hash.
func tryExactMatch(doc *Document, t Thread) (AnchorResult, bool) {
	anchorPath := joinHeadingPath(t.Anchor.HeadingPath)

	for _, p := range doc.Paragraphs {
		if p.Index == t.Anchor.ParagraphIndex &&
			p.HeadingPath == anchorPath &&
			p.ContentHash == t.Anchor.ContentHash {
			return AnchorResult{
				ThreadID: t.ID,
				Status:   AnchorExact,
				Anchor:   t.Anchor,
			}, true
		}
	}

	return AnchorResult{}, false
}

// tryStructuralMatch looks for a paragraph near the original index (±structuralWindow)
// under the same heading path whose content contains the excerpt.
func tryStructuralMatch(doc *Document, t Thread) (AnchorResult, bool) {
	if t.Anchor.Excerpt == "" {
		return AnchorResult{}, false
	}

	anchorPath := joinHeadingPath(t.Anchor.HeadingPath)
	normalizedExcerpt := normalizeForMatch(t.Anchor.Excerpt)

	for _, p := range doc.Paragraphs {
		if p.HeadingPath != anchorPath {
			continue
		}
		if abs(p.Index-t.Anchor.ParagraphIndex) > structuralWindow {
			continue
		}
		normalizedContent := normalizeForMatch(p.Content)
		if strings.Contains(normalizedContent, normalizedExcerpt) {
			return AnchorResult{
				ThreadID: t.ID,
				Status:   AnchorStructural,
				Anchor:   reanchor(t.Anchor, p),
			}, true
		}
	}

	return AnchorResult{}, false
}

// tryFuzzyMatch searches all paragraphs in the document for the excerpt,
// regardless of heading path or paragraph index.
func tryFuzzyMatch(doc *Document, t Thread) (AnchorResult, bool) {
	if t.Anchor.Excerpt == "" {
		return AnchorResult{}, false
	}

	normalizedExcerpt := normalizeForMatch(t.Anchor.Excerpt)

	for _, p := range doc.Paragraphs {
		normalizedContent := normalizeForMatch(p.Content)

		if !strings.Contains(normalizedContent, normalizedExcerpt) {
			continue
		}

		// Check the match ratio to prevent false positives.
		if len(normalizedContent) == 0 {
			continue
		}
		ratio := float64(len(normalizedExcerpt)) / float64(len(normalizedContent))
		if ratio < minFuzzyRatio {
			continue
		}

		return AnchorResult{
			ThreadID: t.ID,
			Status:   AnchorFuzzy,
			Anchor:   reanchor(t.Anchor, p),
		}, true
	}

	return AnchorResult{}, false
}

// reanchor returns a copy of the original anchor updated to reflect a new
// paragraph location. The excerpt and content_hash are updated; char_range
// and source_ref are preserved from the original anchor.
func reanchor(orig Anchor, p *Paragraph) Anchor {
	return Anchor{
		HeadingPath:    splitHeadingPath(p.HeadingPath),
		ParagraphIndex: p.Index,
		Excerpt:        orig.Excerpt,
		ContentHash:    p.ContentHash,
		CharRange:      orig.CharRange,
		SourceRef:      orig.SourceRef,
	}
}

// joinHeadingPath converts a slice of heading segments into the
// slash-separated format used by the Paragraph.HeadingPath field.
func joinHeadingPath(parts []string) string {
	return strings.Join(parts, "/")
}

// splitHeadingPath converts a slash-separated heading path back into a
// slice of heading segments. An empty string returns nil.
func splitHeadingPath(path string) []string {
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

// normalizeForMatch prepares text for substring matching by lower-casing
// and collapsing whitespace.
func normalizeForMatch(s string) string {
	lower := strings.ToLower(s)
	fields := strings.Fields(lower)
	return strings.Join(fields, " ")
}

// abs returns the absolute value of an integer.
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
