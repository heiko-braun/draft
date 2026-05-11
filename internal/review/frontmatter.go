package review

import (
	"bytes"

	"gopkg.in/yaml.v3"
)

// frontMatterDelimiter is the YAML front-matter boundary marker.
var frontMatterDelimiter = []byte("---\n")

// ExtractFrontMatter parses YAML front-matter from markdown content.
// It returns the parsed FrontMatter and the remaining content after the
// closing delimiter. If no valid front-matter is found, it returns an
// empty FrontMatter and the original content unchanged.
func ExtractFrontMatter(content []byte) (FrontMatter, []byte) {
	var fm FrontMatter

	if !bytes.HasPrefix(content, frontMatterDelimiter) {
		return fm, content
	}

	// Search for the closing --- after the opening one.
	rest := content[len(frontMatterDelimiter):]
	idx := bytes.Index(rest, []byte("\n---"))
	if idx < 0 {
		return fm, content
	}

	yamlBlock := rest[:idx]

	// Determine where the body starts (skip the closing --- and newline).
	bodyStart := len(frontMatterDelimiter) + idx + len("\n---")
	if bodyStart < len(content) && content[bodyStart] == '\n' {
		bodyStart++
	}

	if err := yaml.Unmarshal(yamlBlock, &fm); err != nil {
		return FrontMatter{}, content
	}

	return fm, content[bodyStart:]
}
