package review

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// mdParser is a shared goldmark instance used for AST parsing.
var mdParser = goldmark.New()

// IndexDocuments scans the given paths (relative to root) for .md/.mdx files,
// parses each one, and returns a populated DocIndex. Each entry in paths may
// be a file or directory; directories are walked recursively.
func IndexDocuments(root string, paths []string) (*DocIndex, error) {
	idx := &DocIndex{
		Documents: make(map[string]*Document),
	}

	files, err := discoverFiles(root, paths)
	if err != nil {
		return nil, fmt.Errorf("discover files: %w", err)
	}

	for _, relPath := range files {
		absPath := filepath.Join(root, relPath)
		content, err := os.ReadFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", relPath, err)
		}

		doc, err := parseDocument(relPath, content)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", relPath, err)
		}

		idx.Documents[relPath] = doc
	}

	return idx, nil
}

// discoverFiles walks each path (relative to root) and collects .md/.mdx files.
func discoverFiles(root string, paths []string) ([]string, error) {
	var files []string
	seen := make(map[string]bool)

	for _, p := range paths {
		abs := filepath.Join(root, p)
		info, err := os.Stat(abs)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", p, err)
		}

		if !info.IsDir() {
			if isMarkdown(p) && !seen[p] {
				files = append(files, p)
				seen[p] = true
			}
			continue
		}

		err = filepath.WalkDir(abs, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			if isMarkdown(rel) && !seen[rel] {
				files = append(files, rel)
				seen[rel] = true
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", p, err)
		}
	}

	return files, nil
}

// isMarkdown returns true for .md and .mdx file extensions.
func isMarkdown(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md" || ext == ".mdx"
}

// parseDocument parses a single markdown file into a Document.
func parseDocument(relPath string, content []byte) (*Document, error) {
	fm, body := ExtractFrontMatter(content)

	// Parse the body (after front-matter) into a goldmark AST.
	reader := text.NewReader(body)
	tree := mdParser.Parser().Parse(reader)

	doc := &Document{
		RelPath:     relPath,
		FrontMatter: fm,
	}

	// Build heading tree and paragraph list from AST.
	headings, paragraphs := walkAST(tree, body)
	doc.Headings = headings
	doc.Paragraphs = paragraphs

	// Determine title: prefer front-matter, fall back to first heading.
	doc.Title = fm.Title
	if doc.Title == "" && len(headings) > 0 {
		doc.Title = headings[0].Text
	}

	return doc, nil
}

// flatHeading is an intermediate representation used during AST walking.
type flatHeading struct {
	node  *HeadingNode
	level int
}

// walkAST traverses the goldmark AST and extracts headings and paragraphs.
func walkAST(tree ast.Node, source []byte) ([]*HeadingNode, []*Paragraph) {
	var flatHeadings []flatHeading

	var paragraphs []*Paragraph
	paraIndex := 0

	for child := tree.FirstChild(); child != nil; child = child.NextSibling() {
		switch n := child.(type) {
		case *ast.Heading:
			h := &HeadingNode{
				Text:        extractNodeText(n, source),
				Level:       n.Level,
				StartOffset: nodeStartOffset(n),
				EndOffset:   nodeEndOffset(n, source),
			}
			flatHeadings = append(flatHeadings, flatHeading{node: h, level: n.Level})

		case *ast.Paragraph:
			content := extractNodeText(n, source)
			if content == "" {
				continue
			}
			normalized := normalizeText(content)
			hash := sha256Hex(normalized)

			p := &Paragraph{
				Content:     content,
				Index:       paraIndex,
				StartOffset: nodeStartOffset(n),
				EndOffset:   nodeEndOffset(n, source),
				ContentHash: hash,
			}
			paraIndex++
			paragraphs = append(paragraphs, p)
		}
	}

	// Build the heading tree from the flat list.
	roots := buildHeadingTree(flatHeadings)

	// Assign heading end offsets based on subsequent headings / end of source.
	assignHeadingEndOffsets(flatHeadings, len(source))

	// Assign heading paths to paragraphs.
	assignHeadingPaths(paragraphs, flatHeadings)

	return roots, paragraphs
}

// buildHeadingTree nests flat headings into a tree based on heading levels.
func buildHeadingTree(flat []flatHeading) []*HeadingNode {
	var roots []*HeadingNode
	// stack tracks the nesting context: each entry is a parent at a given level.
	var stack []flatHeading

	for _, fh := range flat {
		// Pop stack until we find a parent with a lower level.
		for len(stack) > 0 && stack[len(stack)-1].level >= fh.level {
			stack = stack[:len(stack)-1]
		}

		if len(stack) == 0 {
			roots = append(roots, fh.node)
		} else {
			parent := stack[len(stack)-1].node
			parent.Children = append(parent.Children, fh.node)
		}

		stack = append(stack, fh)
	}

	return roots
}

// assignHeadingEndOffsets sets each heading's EndOffset to the start of the
// next heading at the same or higher level, or the end of the source.
func assignHeadingEndOffsets(flat []flatHeading, sourceLen int) {
	for i, fh := range flat {
		endOffset := sourceLen
		for j := i + 1; j < len(flat); j++ {
			if flat[j].level <= fh.level {
				endOffset = flat[j].node.StartOffset
				break
			}
		}
		fh.node.EndOffset = endOffset
	}
}

// assignHeadingPaths determines the heading path for each paragraph
// based on which heading section it falls under.
func assignHeadingPaths(paragraphs []*Paragraph, flat []flatHeading) {
	for _, p := range paragraphs {
		p.HeadingPath = headingPathForOffset(p.StartOffset, flat)
	}
}

// headingPathForOffset finds the deepest heading containing the given offset
// and returns the slash-separated path of heading texts.
func headingPathForOffset(offset int, flat []flatHeading) string {
	// Walk the flat list and track the current heading path stack.
	var pathStack []string

	for _, fh := range flat {
		if fh.node.StartOffset > offset {
			break
		}
		// Pop headings at the same or deeper level.
		for len(pathStack) > 0 && fh.level <= len(pathStack) {
			pathStack = pathStack[:len(pathStack)-1]
		}
		pathStack = append(pathStack, fh.node.Text)
	}

	return strings.Join(pathStack, "/")
}

// extractNodeText extracts the concatenated plain text from an AST node.
func extractNodeText(n ast.Node, source []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := child.(type) {
		case *ast.Text:
			b.Write(t.Segment.Value(source))
			if t.SoftLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(t.Value)
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

// nodeStartOffset returns the byte offset of the first line of a node.
func nodeStartOffset(n ast.Node) int {
	if n.Lines().Len() > 0 {
		return n.Lines().At(0).Start
	}
	// For nodes without lines (e.g. headings), check first child.
	if c := n.FirstChild(); c != nil {
		return nodeStartOffset(c)
	}
	return 0
}

// nodeEndOffset returns the byte offset of the end of the last line of a node.
func nodeEndOffset(n ast.Node, source []byte) int {
	if n.Lines().Len() > 0 {
		return n.Lines().At(n.Lines().Len() - 1).Stop
	}
	// Walk children to find the furthest end.
	maxEnd := 0
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		end := nodeEndOffset(c, source)
		if end > maxEnd {
			maxEnd = end
		}
	}
	return maxEnd
}

// normalizeText collapses whitespace and trims the text for hashing.
func normalizeText(s string) string {
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// sha256Hex returns the lowercase hex SHA-256 digest of s.
func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}
