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
// Anchor locates a comment within a document using character offsets
// in the rendered text content, with an excerpt for fallback matching.
type Anchor struct {
	// FileHash is the SHA-256 of the file content when the anchor was created.
	// Used to detect whether offsets are still valid.
	FileHash string `json:"file_hash"`

	// Start is the character offset where the selection begins in the
	// rendered text content (element.textContent).
	Start int `json:"start"`

	// End is the character offset where the selection ends.
	End int `json:"end"`

	// Excerpt is the selected text. Used for display and as a fallback
	// to re-locate the annotation when the file has changed.
	Excerpt string `json:"excerpt"`
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
