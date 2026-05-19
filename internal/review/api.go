package review

// --- Interfaces ---

// ReviewStore is the interface the local review server uses for thread/comment/review operations.
type ReviewStore interface {
	ListThreadsByDocument(document string) ([]Thread, error)
	ListAllThreads() ([]Thread, error)
	CreateThread(reviewID, document string, anchor Anchor) (*Thread, error)
	GetThread(document, threadID string) (*Thread, error)
	AddComment(document, threadID, author, body string) (*Comment, error)
	ResolveThread(document, threadID string) error
	ReopenThread(document, threadID string) error
	DeleteThread(document, threadID string) error
	ListReviews() ([]Review, error)
	ListOpenReviews() ([]Review, error)
	CreateReview(title string, documents []string, sourceRef string) (*Review, error)
}

// --- Request types ---

// CreateReviewRequest is the JSON body for POST /api/reviews.
type CreateReviewRequest struct {
	Title     string   `json:"title"`
	Documents []string `json:"documents"`
	SourceRef string   `json:"source_ref,omitempty"`
}

// CreateThreadRequest is the JSON body for POST /api/threads.
type CreateThreadRequest struct {
	ReviewID string `json:"review_id"`
	Document string `json:"document"`
	Anchor   Anchor `json:"anchor"`
	Body     string `json:"body"`
	Author   string `json:"author"`
}

// AddCommentRequest is the JSON body for POST /api/threads/{id}/comments.
type AddCommentRequest struct {
	Author string `json:"author"`
	Body   string `json:"body"`
}

// --- Response types ---

// DocumentListItem is returned by GET /api/documents.
type DocumentListItem struct {
	Path        string `json:"path"`
	Title       string `json:"title"`
	Status      string `json:"status,omitempty"`
	ThreadCount int    `json:"thread_count"`
	ModTime     int64  `json:"mod_time"` // Unix timestamp of last modification
}

// DocumentDetail is returned by GET /api/documents/{path}.
type DocumentDetail struct {
	Path     string      `json:"path"`
	Title    string      `json:"title"`
	HTML     string      `json:"html"`
	FileHash string      `json:"file_hash"`
	Threads  []Thread    `json:"threads"`
	Metadata FrontMatter `json:"metadata"`
}

// StatusResponse is returned by GET /api/status.
type StatusResponse struct {
	RepoName     string `json:"repo_name"`
	Branch       string `json:"branch"`
	OpenReviews  int    `json:"open_reviews"`
	OpenThreads  int    `json:"open_threads"`
	TotalThreads int    `json:"total_threads"`
}
