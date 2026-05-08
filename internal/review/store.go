package review

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Store provides file-based CRUD operations for review data.
// All data is stored as JSON files within the reviews worktree.
type Store struct {
	// root is the absolute path to the reviews worktree root.
	root string
}

// NewStore creates a Store rooted at the given reviews worktree path.
func NewStore(reviewsRoot string) *Store {
	return &Store{root: reviewsRoot}
}

// --- UUID generation ---

// newUUID generates a random UUID v4 string using crypto/rand.
func newUUID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generating UUID: %w", err)
	}
	// Set version (4) and variant (RFC 4122) bits.
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16]), nil
}

// --- Path helpers ---

func (s *Store) reviewPath(reviewID string) string {
	return filepath.Join(s.root, "reviews", reviewID+".json")
}

func (s *Store) threadDir(docPath string) string {
	return filepath.Join(s.root, "threads", docPath)
}

func (s *Store) threadPath(docPath, threadID string) string {
	return filepath.Join(s.root, "threads", docPath, threadID+".json")
}

func (s *Store) participantPath(participantID string) string {
	return filepath.Join(s.root, "participants", participantID+".json")
}

// --- JSON helpers ---

func readJSON(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func writeJSON(path string, v interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

// --- Review CRUD ---

// CreateReview creates a new review and writes it to disk. It returns the
// created Review with a generated ID and timestamps.
func (s *Store) CreateReview(title string, documents []string, sourceRef string) (*Review, error) {
	id, err := newUUID()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	r := &Review{
		ID:        id,
		Title:     title,
		Status:    ReviewOpen,
		Documents: documents,
		SourceRef: sourceRef,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := writeJSON(s.reviewPath(id), r); err != nil {
		return nil, fmt.Errorf("writing review: %w", err)
	}
	return r, nil
}

// GetReview reads a review by ID from disk.
func (s *Store) GetReview(reviewID string) (*Review, error) {
	var r Review
	if err := readJSON(s.reviewPath(reviewID), &r); err != nil {
		return nil, fmt.Errorf("reading review %s: %w", reviewID, err)
	}
	return &r, nil
}

// UpdateReviewStatus changes the status of a review and updates the timestamp.
func (s *Store) UpdateReviewStatus(reviewID string, status ReviewStatus) error {
	r, err := s.GetReview(reviewID)
	if err != nil {
		return err
	}
	r.Status = status
	r.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return writeJSON(s.reviewPath(reviewID), r)
}

// ListReviews returns all reviews on disk.
func (s *Store) ListReviews() ([]Review, error) {
	dir := filepath.Join(s.root, "reviews")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing reviews: %w", err)
	}

	var reviews []Review
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		var r Review
		if err := readJSON(filepath.Join(dir, e.Name()), &r); err != nil {
			continue // skip unreadable files
		}
		reviews = append(reviews, r)
	}
	return reviews, nil
}

// ListOpenReviews returns all reviews with status ReviewOpen.
func (s *Store) ListOpenReviews() ([]Review, error) {
	all, err := s.ListReviews()
	if err != nil {
		return nil, err
	}
	var open []Review
	for _, r := range all {
		if r.Status == ReviewOpen {
			open = append(open, r)
		}
	}
	return open, nil
}

// AddReviewer adds a reviewer entry to the review. If the reviewer already
// exists, it is a no-op.
func (s *Store) AddReviewer(reviewID, participantID string) error {
	r, err := s.GetReview(reviewID)
	if err != nil {
		return err
	}
	for _, rev := range r.Reviewers {
		if rev.ParticipantID == participantID {
			return nil // already assigned
		}
	}
	r.Reviewers = append(r.Reviewers, ReviewerEntry{
		ParticipantID: participantID,
		Status:        "pending",
	})
	r.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return writeJSON(s.reviewPath(reviewID), r)
}

// RemoveReviewer removes a reviewer from a review by participant ID.
func (s *Store) RemoveReviewer(reviewID, participantID string) error {
	r, err := s.GetReview(reviewID)
	if err != nil {
		return err
	}
	filtered := make([]ReviewerEntry, 0, len(r.Reviewers))
	for _, rev := range r.Reviewers {
		if rev.ParticipantID != participantID {
			filtered = append(filtered, rev)
		}
	}
	r.Reviewers = filtered
	r.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return writeJSON(s.reviewPath(reviewID), r)
}

// --- Thread CRUD ---

// CreateThread creates a new thread attached to a document and writes it to
// disk. It returns the created Thread with a generated ID and timestamps.
func (s *Store) CreateThread(reviewID, document string, anchor Anchor) (*Thread, error) {
	id, err := newUUID()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	t := &Thread{
		ID:        id,
		Document:  document,
		Anchor:    anchor,
		ReviewID:  reviewID,
		Status:    ThreadOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := writeJSON(s.threadPath(document, id), t); err != nil {
		return nil, fmt.Errorf("writing thread: %w", err)
	}
	return t, nil
}

// GetThread reads a thread by document path and thread ID.
func (s *Store) GetThread(document, threadID string) (*Thread, error) {
	var t Thread
	if err := readJSON(s.threadPath(document, threadID), &t); err != nil {
		return nil, fmt.Errorf("reading thread %s/%s: %w", document, threadID, err)
	}
	return &t, nil
}

// AddComment adds a comment to a thread and updates the timestamp.
func (s *Store) AddComment(document, threadID, author, body string) (*Comment, error) {
	t, err := s.GetThread(document, threadID)
	if err != nil {
		return nil, err
	}

	commentID, err := newUUID()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	c := Comment{
		ID:        commentID,
		Author:    author,
		Body:      body,
		CreatedAt: now,
	}

	t.Comments = append(t.Comments, c)
	t.UpdatedAt = now

	if err := writeJSON(s.threadPath(document, threadID), t); err != nil {
		return nil, fmt.Errorf("writing thread: %w", err)
	}
	return &c, nil
}

// ResolveThread marks a thread as resolved.
func (s *Store) ResolveThread(document, threadID string) error {
	return s.setThreadStatus(document, threadID, ThreadResolved)
}

// ReopenThread marks a thread as open again.
func (s *Store) ReopenThread(document, threadID string) error {
	return s.setThreadStatus(document, threadID, ThreadOpen)
}

func (s *Store) setThreadStatus(document, threadID string, status ThreadStatus) error {
	t, err := s.GetThread(document, threadID)
	if err != nil {
		return err
	}
	t.Status = status
	t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return writeJSON(s.threadPath(document, threadID), t)
}

// ListThreadsByDocument returns all threads for a given document path.
func (s *Store) ListThreadsByDocument(document string) ([]Thread, error) {
	dir := s.threadDir(document)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing threads for %s: %w", document, err)
	}

	var threads []Thread
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		var t Thread
		if err := readJSON(filepath.Join(dir, e.Name()), &t); err != nil {
			continue
		}
		threads = append(threads, t)
	}
	return threads, nil
}

// ListAllThreads returns all threads across all documents.
func (s *Store) ListAllThreads() ([]Thread, error) {
	threadsRoot := filepath.Join(s.root, "threads")
	var threads []Thread

	err := filepath.Walk(threadsRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}
		var t Thread
		if err := readJSON(path, &t); err != nil {
			return nil // skip unreadable files
		}
		threads = append(threads, t)
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing all threads: %w", err)
	}
	return threads, nil
}

// UpdateThreadAnchor updates the anchor of a thread (e.g. after re-anchoring).
func (s *Store) UpdateThreadAnchor(document, threadID string, anchor Anchor) error {
	t, err := s.GetThread(document, threadID)
	if err != nil {
		return err
	}
	t.Anchor = anchor
	t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return writeJSON(s.threadPath(document, threadID), t)
}

// --- Participant CRUD ---

// participantIDFromEmail derives a stable participant ID from an email address.
func participantIDFromEmail(email string) string {
	h := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	return fmt.Sprintf("%x", h[:8]) // 16 hex chars
}

// EnsureParticipant creates a participant if one with the given ID does not
// already exist. If the participant already exists, it returns the existing one.
func (s *Store) EnsureParticipant(id, name, email string) (*Participant, error) {
	path := s.participantPath(id)
	var existing Participant
	if err := readJSON(path, &existing); err == nil {
		return &existing, nil
	}

	p := &Participant{
		ID:    id,
		Name:  name,
		Email: email,
	}
	if err := writeJSON(path, p); err != nil {
		return nil, fmt.Errorf("writing participant: %w", err)
	}
	return p, nil
}

// GetParticipant reads a participant by ID.
func (s *Store) GetParticipant(participantID string) (*Participant, error) {
	var p Participant
	if err := readJSON(s.participantPath(participantID), &p); err != nil {
		return nil, fmt.Errorf("reading participant %s: %w", participantID, err)
	}
	return &p, nil
}

// EnsureParticipantFromGit creates or retrieves a participant using the
// current git user.name and user.email configuration. This is used to
// auto-register the local user on their first review action.
func (s *Store) EnsureParticipantFromGit() (*Participant, error) {
	name, err := gitConfigValue("user.name")
	if err != nil {
		return nil, fmt.Errorf("reading git user.name: %w", err)
	}
	email, err := gitConfigValue("user.email")
	if err != nil {
		return nil, fmt.Errorf("reading git user.email: %w", err)
	}

	id := participantIDFromEmail(email)
	return s.EnsureParticipant(id, name, email)
}

// gitConfigValue reads a value from git config.
func gitConfigValue(key string) (string, error) {
	cmd := exec.Command("git", "config", "--get", key)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git config %s: %w", key, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// --- Semantic merge ---

// MergeThreads performs a semantic merge of two versions of the same thread.
// It combines comment arrays by ID (dedup), orders by timestamp, and takes
// the latest status based on UpdatedAt.
func MergeThreads(ours, theirs *Thread) *Thread {
	merged := *ours

	// Pick the latest status based on UpdatedAt.
	if theirs.UpdatedAt > ours.UpdatedAt {
		merged.Status = theirs.Status
		merged.UpdatedAt = theirs.UpdatedAt
	}

	// Merge comments: combine by ID, dedup, order by timestamp.
	commentMap := make(map[string]Comment)
	for _, c := range ours.Comments {
		commentMap[c.ID] = c
	}
	for _, c := range theirs.Comments {
		existing, ok := commentMap[c.ID]
		if !ok {
			commentMap[c.ID] = c
		} else {
			// If both have the same comment, keep the one with the later update.
			theirUpdated := c.UpdatedAt
			if theirUpdated == "" {
				theirUpdated = c.CreatedAt
			}
			ourUpdated := existing.UpdatedAt
			if ourUpdated == "" {
				ourUpdated = existing.CreatedAt
			}
			if theirUpdated > ourUpdated {
				commentMap[c.ID] = c
			}
		}
	}

	// Collect and sort by CreatedAt.
	merged.Comments = make([]Comment, 0, len(commentMap))
	for _, c := range commentMap {
		merged.Comments = append(merged.Comments, c)
	}
	sort.Slice(merged.Comments, func(i, j int) bool {
		return merged.Comments[i].CreatedAt < merged.Comments[j].CreatedAt
	})

	return &merged
}
