package review

// DocIndex holds the complete in-memory index of all discovered documents.
type DocIndex struct {
	// Documents keyed by their relative path from the root.
	Documents map[string]*Document
}

// Document represents a single indexed markdown file.
type Document struct {
	// RelPath is the file path relative to the scan root.
	RelPath string

	// Title extracted from front-matter or first heading.
	Title string

	// FrontMatter holds parsed YAML front-matter fields.
	FrontMatter FrontMatter

	// Headings is the top-level heading tree (may contain nested children).
	Headings []*HeadingNode

	// Paragraphs lists all paragraphs in document order.
	Paragraphs []*Paragraph
}

// FrontMatter holds standard YAML front-matter fields.
type FrontMatter struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Status      string `yaml:"status"`
	Author      string `yaml:"author"`
}

// HeadingNode represents a heading in the document hierarchy.
type HeadingNode struct {
	// Text is the plain-text content of the heading.
	Text string

	// Level is the heading depth (1 for #, 2 for ##, etc.).
	Level int

	// StartOffset is the byte offset where this heading starts in the source.
	StartOffset int

	// EndOffset is the byte offset where this heading's section ends in the source.
	EndOffset int

	// Children are nested sub-headings.
	Children []*HeadingNode
}

// Paragraph represents a single paragraph block within a document.
type Paragraph struct {
	// Content is the plain text of the paragraph (inline markup stripped).
	Content string

	// HeadingPath is the slash-separated path of containing headings
	// (e.g. "Goal" or "Approach/Indexing").
	HeadingPath string

	// Index is the zero-based position of this paragraph among all paragraphs in the document.
	Index int

	// StartOffset is the byte offset where the paragraph starts in the source.
	StartOffset int

	// EndOffset is the byte offset where the paragraph ends in the source.
	EndOffset int

	// ContentHash is the SHA-256 hex digest of the normalized paragraph text.
	ContentHash string
}

// Anchor describes a precise location within a document that a review thread
// is attached to. The fields record enough information to re-locate the
// anchored content even after the document has been edited.
type Anchor struct {
	// HeadingPath is the sequence of heading texts from the document root
	// to the section containing the anchored content (e.g. ["Goal", "Sub-goal"]).
	HeadingPath []string `json:"heading_path"`

	// ParagraphIndex is the zero-based index of the paragraph within the document.
	ParagraphIndex int `json:"paragraph_index"`

	// Excerpt is a representative snippet of the anchored content used for
	// structural and fuzzy matching when the content hash no longer matches.
	Excerpt string `json:"excerpt"`

	// ContentHash is the SHA-256 hex digest of the normalized paragraph text
	// at the time the anchor was created.
	ContentHash string `json:"content_hash"`

	// CharRange marks the start and end character offsets within the paragraph.
	CharRange [2]int `json:"char_range"`

	// SourceRef is an opaque reference to the source location (e.g. file path
	// and line number) for display purposes.
	SourceRef string `json:"source_ref"`
}

// ThreadStatus represents the resolution status of a review thread.
type ThreadStatus string

const (
	// ThreadOpen indicates the thread is open and unresolved.
	ThreadOpen ThreadStatus = "open"

	// ThreadResolved indicates the thread has been resolved.
	ThreadResolved ThreadStatus = "resolved"

	// ThreadWontFix indicates the thread was closed without a fix.
	ThreadWontFix ThreadStatus = "wontfix"
)

// ReviewStatus represents the overall status of a review.
type ReviewStatus string

const (
	// ReviewOpen indicates the review is active and accepting feedback.
	ReviewOpen ReviewStatus = "open"

	// ReviewClosed indicates the review was closed without merging.
	ReviewClosed ReviewStatus = "closed"

	// ReviewMerged indicates the review was completed and merged.
	ReviewMerged ReviewStatus = "merged"
)

// Thread is a representation of a review thread, carrying enough
// information for anchor resolution and full review data.
type Thread struct {
	// ID uniquely identifies the thread.
	ID string `json:"id"`

	// Document is the relative path of the document this thread is attached to.
	Document string `json:"document"`

	// Anchor is the location within the document.
	Anchor Anchor `json:"anchor"`

	// ReviewID links the thread to a specific review.
	ReviewID string `json:"review_id,omitempty"`

	// Status is the resolution status of the thread.
	Status ThreadStatus `json:"status,omitempty"`

	// Comments holds the discussion on this thread.
	Comments []Comment `json:"comments,omitempty"`

	// CreatedAt is the RFC 3339 timestamp when the thread was created.
	CreatedAt string `json:"created_at,omitempty"`

	// UpdatedAt is the RFC 3339 timestamp when the thread was last modified.
	UpdatedAt string `json:"updated_at,omitempty"`
}

// Comment is a single entry in a thread's discussion.
type Comment struct {
	// ID uniquely identifies the comment (UUID for merge deduplication).
	ID string `json:"id"`

	// Author is the participant ID of the comment author.
	Author string `json:"author"`

	// Body is the comment text.
	Body string `json:"body"`

	// CreatedAt is the RFC 3339 timestamp when the comment was created.
	CreatedAt string `json:"created_at"`

	// UpdatedAt is the RFC 3339 timestamp when the comment was last modified.
	UpdatedAt string `json:"updated_at,omitempty"`
}

// Review represents a document review containing threads and metadata.
type Review struct {
	// ID uniquely identifies the review.
	ID string `json:"id"`

	// Title is the human-readable title of the review.
	Title string `json:"title"`

	// Status is the overall review status.
	Status ReviewStatus `json:"status"`

	// Documents lists the relative paths of documents under review.
	Documents []string `json:"documents"`

	// Reviewers lists the assigned reviewers and their status.
	Reviewers []ReviewerEntry `json:"reviewers,omitempty"`

	// SourceRef is an opaque reference to the source version (e.g. commit SHA)
	// used for stale approval detection.
	SourceRef string `json:"source_ref,omitempty"`

	// CreatedAt is the RFC 3339 timestamp when the review was created.
	CreatedAt string `json:"created_at"`

	// UpdatedAt is the RFC 3339 timestamp when the review was last modified.
	UpdatedAt string `json:"updated_at"`
}

// ReviewerEntry records a reviewer assignment on a review.
type ReviewerEntry struct {
	// ParticipantID is the participant who is assigned as reviewer.
	ParticipantID string `json:"participant_id"`

	// Status is the reviewer's current verdict (e.g. "pending", "approved", "changes_requested").
	Status string `json:"status"`

	// ApprovedAt is the RFC 3339 timestamp of the approval, if any.
	ApprovedAt string `json:"approved_at,omitempty"`

	// ApprovalSourceRef is the source_ref at the time of approval,
	// used for stale approval detection.
	ApprovalSourceRef string `json:"approval_source_ref,omitempty"`
}

// Participant represents a user participating in reviews.
type Participant struct {
	// ID is a stable hash-based identifier derived from email or name.
	ID string `json:"id"`

	// Name is the display name of the participant.
	Name string `json:"name"`

	// Email is the email address of the participant.
	Email string `json:"email"`
}

// AnchorStatus describes the outcome of attempting to resolve a thread's
// anchor against the current document index.
type AnchorStatus int

const (
	// AnchorExact means the content hash matched the paragraph at the
	// expected heading path and index.
	AnchorExact AnchorStatus = iota

	// AnchorStructural means the heading path exists and a nearby paragraph
	// (within ±3 positions) contains the excerpt.
	AnchorStructural

	// AnchorFuzzy means the excerpt was found in a different paragraph
	// elsewhere in the document.
	AnchorFuzzy

	// AnchorOrphaned means no match could be found; the thread's original
	// anchor is preserved for display context.
	AnchorOrphaned
)

// String returns a human-readable label for the AnchorStatus.
func (s AnchorStatus) String() string {
	switch s {
	case AnchorExact:
		return "exact"
	case AnchorStructural:
		return "structural"
	case AnchorFuzzy:
		return "fuzzy"
	case AnchorOrphaned:
		return "orphaned"
	default:
		return "unknown"
	}
}

// AnchorResult captures the outcome of resolving a single thread's anchor
// against the current document state.
type AnchorResult struct {
	// ThreadID is the ID of the thread that was resolved.
	ThreadID string

	// Status indicates how the anchor was matched.
	Status AnchorStatus

	// Anchor is the (possibly updated) anchor. For exact matches it is
	// unchanged; for structural/fuzzy matches it reflects the new location;
	// for orphaned threads it preserves the original anchor.
	Anchor Anchor
}
