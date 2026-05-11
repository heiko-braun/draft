package reviewd

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/heiko-braun/draft/internal/review"
)

// Store provides Postgres-backed CRUD operations for review entities.
type Store struct {
	db *sql.DB
}

// NewStore creates a new Store.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// --- Repos ---

// Repo represents a registered repository.
type Repo struct {
	ID          string    `json:"id"`
	GitHubOwner string    `json:"github_owner"`
	GitHubRepo  string    `json:"github_repo"`
	CreatedAt   time.Time `json:"created_at"`
}

// GetOrCreateRepo returns the repo record, creating it if it doesn't exist.
func (s *Store) GetOrCreateRepo(owner, repo string) (*Repo, error) {
	r := &Repo{}
	err := s.db.QueryRow(`
		INSERT INTO repos (github_owner, github_repo)
		VALUES ($1, $2)
		ON CONFLICT (github_owner, github_repo) DO UPDATE SET github_owner = EXCLUDED.github_owner
		RETURNING id, github_owner, github_repo, created_at
	`, owner, repo).Scan(&r.ID, &r.GitHubOwner, &r.GitHubRepo, &r.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get or create repo: %w", err)
	}
	return r, nil
}

// GetRepo returns a repo by owner and name.
func (s *Store) GetRepo(owner, repo string) (*Repo, error) {
	r := &Repo{}
	err := s.db.QueryRow(`
		SELECT id, github_owner, github_repo, created_at
		FROM repos WHERE github_owner = $1 AND github_repo = $2
	`, owner, repo).Scan(&r.ID, &r.GitHubOwner, &r.GitHubRepo, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get repo: %w", err)
	}
	return r, nil
}

// --- Participants ---

// GetOrCreateParticipant ensures a participant exists.
func (s *Store) GetOrCreateParticipant(id, name, email string) (*review.Participant, error) {
	p := &review.Participant{}
	err := s.db.QueryRow(`
		INSERT INTO participants (id, name, email)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name
		RETURNING id, name, email
	`, id, name, email).Scan(&p.ID, &p.Name, &p.Email)
	if err != nil {
		return nil, fmt.Errorf("get or create participant: %w", err)
	}
	return p, nil
}

// GetParticipant returns a participant by ID.
func (s *Store) GetParticipant(id string) (*review.Participant, error) {
	p := &review.Participant{}
	err := s.db.QueryRow(`
		SELECT id, name, email FROM participants WHERE id = $1
	`, id).Scan(&p.ID, &p.Name, &p.Email)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get participant: %w", err)
	}
	return p, nil
}

// --- Threads ---

// ThreadRow is a thread with its server-side version for optimistic concurrency.
type ThreadRow struct {
	review.Thread
	RepoID  string `json:"repo_id"`
	Version int    `json:"version"`
}

// CreateThread creates a new thread in the database.
func (s *Store) CreateThread(repoID string, t *review.Thread) (*ThreadRow, error) {
	anchorJSON, err := json.Marshal(t.Anchor)
	if err != nil {
		return nil, fmt.Errorf("marshal anchor: %w", err)
	}

	row := &ThreadRow{}
	var anchorBytes []byte
	err = s.db.QueryRow(`
		INSERT INTO threads (repo_id, review_id, document, anchor, status)
		VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5)
		RETURNING id, repo_id, COALESCE(review_id::text, ''), document, anchor, status, version, created_at, updated_at
	`, repoID, t.ReviewID, t.Document, anchorJSON, statusOrDefault(string(t.Status))).
		Scan(&row.ID, &row.RepoID, &row.ReviewID, &row.Document, &anchorBytes, &row.Status, &row.Version, &row.CreatedAt, &row.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create thread: %w", err)
	}
	if err := json.Unmarshal(anchorBytes, &row.Anchor); err != nil {
		return nil, fmt.Errorf("unmarshal anchor: %w", err)
	}
	return row, nil
}

// GetThread returns a thread by ID, including its comments.
func (s *Store) GetThread(threadID string) (*ThreadRow, error) {
	row := &ThreadRow{}
	var anchorBytes []byte
	err := s.db.QueryRow(`
		SELECT id, repo_id, COALESCE(review_id::text, ''), document, anchor, status, version, created_at, updated_at
		FROM threads WHERE id = $1
	`, threadID).Scan(&row.ID, &row.RepoID, &row.ReviewID, &row.Document, &anchorBytes, &row.Status, &row.Version, &row.CreatedAt, &row.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get thread: %w", err)
	}
	if err := json.Unmarshal(anchorBytes, &row.Anchor); err != nil {
		return nil, fmt.Errorf("unmarshal anchor: %w", err)
	}

	comments, err := s.ListComments(threadID)
	if err != nil {
		return nil, err
	}
	row.Comments = comments
	return row, nil
}

// ListThreadsByDocument returns all threads for a repo and document path.
func (s *Store) ListThreadsByDocument(repoID, document string) ([]ThreadRow, error) {
	rows, err := s.db.Query(`
		SELECT id, repo_id, COALESCE(review_id::text, ''), document, anchor, status, version, created_at, updated_at
		FROM threads WHERE repo_id = $1 AND document = $2
		ORDER BY created_at
	`, repoID, document)
	if err != nil {
		return nil, fmt.Errorf("list threads by document: %w", err)
	}
	defer rows.Close()
	return s.scanThreadRows(rows)
}

// ListThreadsByRepo returns all threads for a repo.
func (s *Store) ListThreadsByRepo(repoID string) ([]ThreadRow, error) {
	rows, err := s.db.Query(`
		SELECT id, repo_id, COALESCE(review_id::text, ''), document, anchor, status, version, created_at, updated_at
		FROM threads WHERE repo_id = $1
		ORDER BY created_at
	`, repoID)
	if err != nil {
		return nil, fmt.Errorf("list threads by repo: %w", err)
	}
	defer rows.Close()
	return s.scanThreadRows(rows)
}

// ListThreadsChangedSince returns threads updated after the given timestamp.
func (s *Store) ListThreadsChangedSince(repoID string, since time.Time) ([]ThreadRow, error) {
	rows, err := s.db.Query(`
		SELECT id, repo_id, COALESCE(review_id::text, ''), document, anchor, status, version, created_at, updated_at
		FROM threads WHERE repo_id = $1 AND updated_at > $2
		ORDER BY updated_at
	`, repoID, since)
	if err != nil {
		return nil, fmt.Errorf("list threads changed since: %w", err)
	}
	defer rows.Close()
	return s.scanThreadRows(rows)
}

// UpdateThreadStatus updates a thread's status with optimistic concurrency.
// Returns ErrVersionConflict if expectedVersion doesn't match.
func (s *Store) UpdateThreadStatus(threadID string, status review.ThreadStatus, expectedVersion int) (*ThreadRow, error) {
	row := &ThreadRow{}
	var anchorBytes []byte
	err := s.db.QueryRow(`
		UPDATE threads SET status = $1, version = version + 1, updated_at = now()
		WHERE id = $2 AND version = $3
		RETURNING id, repo_id, COALESCE(review_id::text, ''), document, anchor, status, version, created_at, updated_at
	`, string(status), threadID, expectedVersion).
		Scan(&row.ID, &row.RepoID, &row.ReviewID, &row.Document, &anchorBytes, &row.Status, &row.Version, &row.CreatedAt, &row.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrVersionConflict
	}
	if err != nil {
		return nil, fmt.Errorf("update thread status: %w", err)
	}
	if err := json.Unmarshal(anchorBytes, &row.Anchor); err != nil {
		return nil, fmt.Errorf("unmarshal anchor: %w", err)
	}
	return row, nil
}

// DeleteThread removes a thread and its comments.
func (s *Store) DeleteThread(threadID string) error {
	res, err := s.db.Exec("DELETE FROM threads WHERE id = $1", threadID)
	if err != nil {
		return fmt.Errorf("delete thread: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) scanThreadRows(rows *sql.Rows) ([]ThreadRow, error) {
	var threads []ThreadRow
	for rows.Next() {
		var row ThreadRow
		var anchorBytes []byte
		if err := rows.Scan(&row.ID, &row.RepoID, &row.ReviewID, &row.Document, &anchorBytes, &row.Status, &row.Version, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan thread: %w", err)
		}
		if err := json.Unmarshal(anchorBytes, &row.Anchor); err != nil {
			return nil, fmt.Errorf("unmarshal anchor: %w", err)
		}
		threads = append(threads, row)
	}
	return threads, rows.Err()
}

// --- Comments ---

// AddComment appends a comment to a thread. This also increments the thread's
// version and updated_at, and auto-reopens resolved threads.
func (s *Store) AddComment(threadID, authorID, body string) (*review.Comment, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Auto-reopen if resolved.
	_, err = tx.Exec(`
		UPDATE threads SET
			status = CASE WHEN status = 'resolved' THEN 'open' ELSE status END,
			version = version + 1,
			updated_at = now()
		WHERE id = $1
	`, threadID)
	if err != nil {
		return nil, fmt.Errorf("update thread for comment: %w", err)
	}

	c := &review.Comment{}
	err = tx.QueryRow(`
		INSERT INTO comments (thread_id, author, body)
		VALUES ($1, $2, $3)
		RETURNING id, author, body, created_at, updated_at
	`, threadID, authorID, body).Scan(&c.ID, &c.Author, &c.Body, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert comment: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return c, nil
}

// ListComments returns all comments for a thread, ordered by created_at.
func (s *Store) ListComments(threadID string) ([]review.Comment, error) {
	rows, err := s.db.Query(`
		SELECT id, author, body, created_at, COALESCE(updated_at::text, '')
		FROM comments WHERE thread_id = $1
		ORDER BY created_at
	`, threadID)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()

	var comments []review.Comment
	for rows.Next() {
		var c review.Comment
		if err := rows.Scan(&c.ID, &c.Author, &c.Body, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

// --- Reviews ---

// CreateReview creates a new review.
func (s *Store) CreateReview(repoID, title string, documents []string, sourceRef string) (*review.Review, error) {
	docsJSON, err := json.Marshal(documents)
	if err != nil {
		return nil, fmt.Errorf("marshal documents: %w", err)
	}

	r := &review.Review{}
	var docsBytes []byte
	err = s.db.QueryRow(`
		INSERT INTO reviews (repo_id, title, documents, source_ref)
		VALUES ($1, $2, $3, $4)
		RETURNING id, title, status, documents, COALESCE(source_ref, ''), created_at, updated_at
	`, repoID, title, docsJSON, sourceRef).
		Scan(&r.ID, &r.Title, &r.Status, &docsBytes, &r.SourceRef, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create review: %w", err)
	}
	json.Unmarshal(docsBytes, &r.Documents)
	return r, nil
}

// GetReview returns a review by ID including reviewers.
func (s *Store) GetReview(reviewID string) (*review.Review, error) {
	r := &review.Review{}
	var docsBytes []byte
	err := s.db.QueryRow(`
		SELECT id, title, status, documents, COALESCE(source_ref, ''), created_at, updated_at
		FROM reviews WHERE id = $1
	`, reviewID).Scan(&r.ID, &r.Title, &r.Status, &docsBytes, &r.SourceRef, &r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get review: %w", err)
	}
	json.Unmarshal(docsBytes, &r.Documents)

	// Load reviewers.
	rows, err := s.db.Query(`
		SELECT participant_id, status, approved_at, COALESCE(approval_source_ref, '')
		FROM review_reviewers WHERE review_id = $1
	`, reviewID)
	if err != nil {
		return nil, fmt.Errorf("list reviewers: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var re review.ReviewerEntry
		var approvedAt sql.NullTime
		if err := rows.Scan(&re.ParticipantID, &re.Status, &approvedAt, &re.ApprovalSourceRef); err != nil {
			return nil, fmt.Errorf("scan reviewer: %w", err)
		}
		if approvedAt.Valid {
			re.ApprovedAt = approvedAt.Time.Format(time.RFC3339)
		}
		r.Reviewers = append(r.Reviewers, re)
	}
	return r, rows.Err()
}

// ListReviewsByRepo returns all reviews for a repo.
func (s *Store) ListReviewsByRepo(repoID string) ([]review.Review, error) {
	rows, err := s.db.Query(`
		SELECT id, title, status, documents, COALESCE(source_ref, ''), created_at, updated_at
		FROM reviews WHERE repo_id = $1
		ORDER BY created_at DESC
	`, repoID)
	if err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}
	defer rows.Close()

	var reviews []review.Review
	for rows.Next() {
		var r review.Review
		var docsBytes []byte
		if err := rows.Scan(&r.ID, &r.Title, &r.Status, &docsBytes, &r.SourceRef, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan review: %w", err)
		}
		json.Unmarshal(docsBytes, &r.Documents)
		reviews = append(reviews, r)
	}
	return reviews, rows.Err()
}

// ReviewBelongsToRepo returns true if the review belongs to the given repo.
func (s *Store) ReviewBelongsToRepo(reviewID, repoID string) (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM reviews WHERE id = $1 AND repo_id = $2", reviewID, repoID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// UpdateReviewStatus updates a review's status.
func (s *Store) UpdateReviewStatus(reviewID string, status review.ReviewStatus) error {
	res, err := s.db.Exec(`
		UPDATE reviews SET status = $1, updated_at = now() WHERE id = $2
	`, string(status), reviewID)
	if err != nil {
		return fmt.Errorf("update review status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// AddReviewer assigns a reviewer to a review.
func (s *Store) AddReviewer(reviewID, participantID string) error {
	_, err := s.db.Exec(`
		INSERT INTO review_reviewers (review_id, participant_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, reviewID, participantID)
	if err != nil {
		return fmt.Errorf("add reviewer: %w", err)
	}
	return nil
}

// RemoveReviewer removes a reviewer from a review.
func (s *Store) RemoveReviewer(reviewID, participantID string) error {
	_, err := s.db.Exec(`
		DELETE FROM review_reviewers WHERE review_id = $1 AND participant_id = $2
	`, reviewID, participantID)
	return err
}

func statusOrDefault(s string) string {
	if s == "" {
		return "open"
	}
	return s
}
