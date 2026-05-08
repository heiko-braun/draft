package review

import "encoding/json"

// SchemaVersion is the current version of the review branch data schema.
const SchemaVersion = "1"

// ReviewConfig holds the configuration stored in config.json on the review branch.
type ReviewConfig struct {
	DocumentPaths []string `json:"document_paths"`
	FilePatterns  []string `json:"file_patterns"`
	DefaultBranch string   `json:"default_branch"`
}

// DefaultConfig returns a ReviewConfig populated with sensible defaults.
func DefaultConfig() ReviewConfig {
	return ReviewConfig{
		DocumentPaths: []string{"specs/", "docs/", "rfcs/", "adrs/"},
		FilePatterns:  []string{"*.md", "*.mdx"},
		DefaultBranch: "main",
	}
}

// MarshalJSON returns the JSON encoding of the config with indentation.
func (c ReviewConfig) MarshalJSON() ([]byte, error) {
	// Use an alias to avoid infinite recursion.
	type Alias ReviewConfig
	return json.MarshalIndent(Alias(c), "", "  ")
}

// ParseConfig deserializes a ReviewConfig from JSON bytes.
func ParseConfig(data []byte) (ReviewConfig, error) {
	var c ReviewConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return ReviewConfig{}, err
	}
	return c, nil
}
