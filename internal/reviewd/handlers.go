package reviewd

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/heiko-braun/draft/internal/review"
)

// --- Thread Handlers ---

func (s *Server) handleListThreads(w http.ResponseWriter, r *http.Request) {
	repo, err := s.ensureRepo(r)
	if err != nil {
		return
	}

	document := r.URL.Query().Get("document")
	status := r.URL.Query().Get("status")
	s.logger.Debug("list threads", "repo", repo.GitHubOwner+"/"+repo.GitHubRepo, "document", document, "status_filter", status)

	var threads []ThreadRow
	if document != "" {
		threads, err = s.store.ListThreadsByDocument(repo.ID, document)
	} else {
		threads, err = s.store.ListThreadsByRepo(repo.ID)
	}
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Filter by status if requested.
	if status != "" && threads != nil {
		filtered := threads[:0]
		for _, t := range threads {
			if string(t.Status) == status {
				filtered = append(filtered, t)
			}
		}
		threads = filtered
	}

	if threads == nil {
		threads = []ThreadRow{}
	}

	// Load comments for each thread.
	for i := range threads {
		comments, _ := s.store.ListComments(threads[i].ID)
		if comments == nil {
			comments = []review.Comment{}
		}
		threads[i].Comments = comments
	}

	writeJSON(w, http.StatusOK, threads)
}

func (s *Server) handleGetThread(w http.ResponseWriter, r *http.Request) {
	repo, err := s.ensureRepo(r)
	if err != nil {
		return
	}
	threadID := r.PathValue("threadID")
	s.logger.Debug("get thread", "thread_id", threadID)
	thread, err := s.store.GetThread(threadID)
	if errors.Is(err, ErrNotFound) || (err == nil && thread.RepoID != repo.ID) {
		writeErrorJSON(w, http.StatusNotFound, "thread not found")
		return
	}
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, thread)
}

// PutThreadRequest is the body for creating or updating a thread.
type PutThreadRequest struct {
	Document string        `json:"document"`
	Anchor   review.Anchor `json:"anchor"`
	ReviewID string        `json:"review_id,omitempty"`
	Status   string        `json:"status,omitempty"`
}

func (s *Server) handlePutThread(w http.ResponseWriter, r *http.Request) {
	repo, err := s.ensureRepo(r)
	if err != nil {
		return
	}

	threadID := r.PathValue("threadID")

	var req PutThreadRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	s.logger.Debug("put thread", "thread_id", threadID, "document", req.Document, "status", req.Status, "if_match", r.Header.Get("If-Match"))

	// Check if thread exists — update vs create.
	existing, err := s.store.GetThread(threadID)
	if errors.Is(err, ErrNotFound) {
		s.logger.Debug("creating new thread", "thread_id", threadID)
		// Create new thread.
		t := &review.Thread{
			ID:       threadID,
			Document: req.Document,
			Anchor:   req.Anchor,
			ReviewID: req.ReviewID,
		}
		created, err := s.store.CreateThread(repo.ID, t)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.broadcastEvent(repo.ID, "thread.created", created)
		writeJSON(w, http.StatusCreated, created)
		return
	}
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Verify thread belongs to this repo.
	if existing.RepoID != repo.ID {
		writeErrorJSON(w, http.StatusNotFound, "thread not found")
		return
	}

	// Update existing — requires If-Match header.
	ifMatch := r.Header.Get("If-Match")
	if ifMatch == "" {
		writeErrorJSON(w, http.StatusPreconditionRequired, "If-Match header required for updates")
		return
	}
	expectedVersion, err := strconv.Atoi(ifMatch)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "If-Match must be an integer version")
		return
	}

	newStatus := review.ThreadStatus(req.Status)
	if newStatus == "" {
		newStatus = existing.Status
	}

	s.logger.Debug("updating thread", "thread_id", threadID, "new_status", string(newStatus), "expected_version", strconv.Itoa(expectedVersion))
	updated, err := s.store.UpdateThreadStatus(threadID, newStatus, expectedVersion)
	if errors.Is(err, ErrVersionConflict) {
		s.logger.Debug("version conflict", "thread_id", threadID, "expected", strconv.Itoa(expectedVersion))
		// Return current state for client merge.
		current, _ := s.store.GetThread(threadID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		writeJSON(w, http.StatusConflict, map[string]interface{}{
			"error":           "version conflict",
			"current_version": current.Version,
			"thread":          current,
		})
		return
	}
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.broadcastEvent(repo.ID, "thread.updated", updated)
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteThread(w http.ResponseWriter, r *http.Request) {
	repo, err := s.ensureRepo(r)
	if err != nil {
		return
	}
	threadID := r.PathValue("threadID")
	s.logger.Debug("delete thread", "thread_id", threadID)

	// Get thread before deleting for the event and repo ownership check.
	thread, err := s.store.GetThread(threadID)
	if errors.Is(err, ErrNotFound) || (err == nil && thread.RepoID != repo.ID) {
		writeErrorJSON(w, http.StatusNotFound, "thread not found")
		return
	}

	if err := s.store.DeleteThread(threadID); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.broadcastEvent(thread.RepoID, "thread.deleted", map[string]string{"id": threadID})
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- Comment Handlers ---

// AddCommentRequest is the body for adding a comment.
type AddCommentAPIRequest struct {
	Body string `json:"body"`
}

func (s *Server) handleAddComment(w http.ResponseWriter, r *http.Request) {
	repo, err := s.ensureRepo(r)
	if err != nil {
		return
	}
	threadID := r.PathValue("threadID")

	// Verify thread belongs to this repo.
	thread, err := s.store.GetThread(threadID)
	if errors.Is(err, ErrNotFound) || (err == nil && thread.RepoID != repo.ID) {
		writeErrorJSON(w, http.StatusNotFound, "thread not found")
		return
	}

	var req AddCommentAPIRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Body == "" {
		writeErrorJSON(w, http.StatusBadRequest, "body is required")
		return
	}

	user := UserFromContext(r.Context())
	authorID := user.ParticipantID
	s.logger.Debug("add comment", "thread_id", threadID, "author", user.GitHubLogin, "body_len", strconv.Itoa(len(req.Body)))

	// Ensure participant exists.
	s.store.GetOrCreateParticipant(authorID, user.Name, user.Email)

	comment, err := s.store.AddComment(threadID, authorID, req.Body)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Re-fetch thread for broadcasting (may have been auto-reopened).
	thread, _ = s.store.GetThread(threadID)
	if thread != nil {
		s.broadcastEvent(thread.RepoID, "comment.created", map[string]interface{}{
			"thread_id": threadID,
			"comment":   comment,
		})
		// If auto-reopened, broadcast that too.
		if string(thread.Status) == "open" {
			s.broadcastEvent(thread.RepoID, "thread.reopened", thread)
		}
	}

	writeJSON(w, http.StatusCreated, comment)
}

// --- Review Handlers ---

// CreateReviewAPIRequest is the body for creating a review.
type CreateReviewAPIRequest struct {
	Title     string   `json:"title"`
	Documents []string `json:"documents"`
	SourceRef string   `json:"source_ref,omitempty"`
}

func (s *Server) handleListReviews(w http.ResponseWriter, r *http.Request) {
	repo, err := s.ensureRepo(r)
	if err != nil {
		return
	}

	reviews, err := s.store.ListReviewsByRepo(repo.ID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if reviews == nil {
		reviews = []review.Review{}
	}
	writeJSON(w, http.StatusOK, reviews)
}

func (s *Server) handleCreateReview(w http.ResponseWriter, r *http.Request) {
	repo, err := s.ensureRepo(r)
	if err != nil {
		return
	}

	var req CreateReviewAPIRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" {
		writeErrorJSON(w, http.StatusBadRequest, "title is required")
		return
	}

	s.logger.Debug("create review", "title", req.Title, "documents", fmt.Sprintf("%v", req.Documents))
	rev, err := s.store.CreateReview(repo.ID, req.Title, req.Documents, req.SourceRef)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, rev)
}

func (s *Server) handleGetReview(w http.ResponseWriter, r *http.Request) {
	repo, err := s.ensureRepo(r)
	if err != nil {
		return
	}
	reviewID := r.PathValue("reviewID")
	belongs, err := s.store.ReviewBelongsToRepo(reviewID, repo.ID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !belongs {
		writeErrorJSON(w, http.StatusNotFound, "review not found")
		return
	}
	rev, err := s.store.GetReview(reviewID)
	if errors.Is(err, ErrNotFound) {
		writeErrorJSON(w, http.StatusNotFound, "review not found")
		return
	}
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rev)
}

// PatchReviewRequest allows updating review status and reviewers.
type PatchReviewRequest struct {
	Status          string   `json:"status,omitempty"`
	AddReviewers    []string `json:"add_reviewers,omitempty"`
	RemoveReviewers []string `json:"remove_reviewers,omitempty"`
}

func (s *Server) handlePatchReview(w http.ResponseWriter, r *http.Request) {
	repo, err := s.ensureRepo(r)
	if err != nil {
		return
	}
	reviewID := r.PathValue("reviewID")
	belongs, err := s.store.ReviewBelongsToRepo(reviewID, repo.ID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !belongs {
		writeErrorJSON(w, http.StatusNotFound, "review not found")
		return
	}

	var req PatchReviewRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Status != "" {
		if err := s.store.UpdateReviewStatus(reviewID, review.ReviewStatus(req.Status)); err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	for _, pid := range req.AddReviewers {
		s.store.AddReviewer(reviewID, pid)
	}
	for _, pid := range req.RemoveReviewers {
		s.store.RemoveReviewer(reviewID, pid)
	}

	rev, err := s.store.GetReview(reviewID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rev)
}

// --- Sync Handlers ---

// SyncRequest is the body for bulk pull.
type SyncRequest struct {
	Since string `json:"since"`
}

// SyncResponse is the response for bulk pull.
type SyncAPIResponse struct {
	Threads    []ThreadRow     `json:"threads"`
	Reviews    []review.Review `json:"reviews"`
	ServerTime string          `json:"server_time"`
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	repo, err := s.ensureRepo(r)
	if err != nil {
		return
	}

	var req SyncRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	s.logger.Debug("sync", "repo", repo.GitHubOwner+"/"+repo.GitHubRepo, "since", req.Since)

	since := time.Time{} // epoch — returns everything
	if req.Since != "" {
		parsed, err := time.Parse(time.RFC3339, req.Since)
		if err != nil {
			writeErrorJSON(w, http.StatusBadRequest, "invalid since timestamp, use RFC3339")
			return
		}
		since = parsed
	}

	threads, err := s.store.ListThreadsChangedSince(repo.ID, since)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if threads == nil {
		threads = []ThreadRow{}
	}

	// Load comments for each thread.
	for i := range threads {
		comments, _ := s.store.ListComments(threads[i].ID)
		if comments == nil {
			comments = []review.Comment{}
		}
		threads[i].Comments = comments
	}

	reviews, err := s.store.ListReviewsByRepo(repo.ID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if reviews == nil {
		reviews = []review.Review{}
	}

	writeJSON(w, http.StatusOK, SyncAPIResponse{
		Threads:    threads,
		Reviews:    reviews,
		ServerTime: time.Now().UTC().Format(time.RFC3339),
	})
}

// PublishMutation is a single mutation in a bulk publish.
type PublishMutation struct {
	Op       string         `json:"op"` // "upsert_thread", "add_comment", "delete_thread", "resolve_thread", "reopen_thread"
	ThreadID string         `json:"thread_id,omitempty"`
	Thread   *review.Thread `json:"thread,omitempty"`
	Comment  *struct {
		Body string `json:"body"`
	} `json:"comment,omitempty"`
	ExpectedVersion int `json:"expected_version,omitempty"`
}

// PublishRequest is the body for bulk push.
type PublishAPIRequest struct {
	Mutations []PublishMutation `json:"mutations"`
}

// MutationResult is the result of a single mutation.
type MutationResult struct {
	Index   int             `json:"index"`
	OK      bool            `json:"ok"`
	Error   string          `json:"error,omitempty"`
	Thread  *ThreadRow      `json:"thread,omitempty"`
	Comment *review.Comment `json:"comment,omitempty"`
}

// PublishAPIResponse is the response for bulk push.
type PublishAPIResponse struct {
	Results []MutationResult `json:"results"`
}

func (s *Server) handlePublish(w http.ResponseWriter, r *http.Request) {
	repo, err := s.ensureRepo(r)
	if err != nil {
		return
	}

	user := UserFromContext(r.Context())

	var req PublishAPIRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	s.logger.Debug("publish", "repo", repo.GitHubOwner+"/"+repo.GitHubRepo, "mutations", strconv.Itoa(len(req.Mutations)), "user", user.GitHubLogin)

	results := make([]MutationResult, len(req.Mutations))
	for i, mut := range req.Mutations {
		results[i].Index = i
		switch mut.Op {
		case "upsert_thread":
			if mut.Thread == nil {
				results[i].Error = "thread is required for upsert_thread"
				continue
			}
			created, err := s.store.CreateThread(repo.ID, mut.Thread)
			if err != nil {
				results[i].Error = err.Error()
				continue
			}
			results[i].OK = true
			results[i].Thread = created

		case "add_comment":
			if mut.ThreadID == "" || mut.Comment == nil {
				results[i].Error = "thread_id and comment are required for add_comment"
				continue
			}
			s.store.GetOrCreateParticipant(user.ParticipantID, user.Name, user.Email)
			comment, err := s.store.AddComment(mut.ThreadID, user.ParticipantID, mut.Comment.Body)
			if err != nil {
				results[i].Error = err.Error()
				continue
			}
			results[i].OK = true
			results[i].Comment = comment

		case "resolve_thread":
			if mut.ThreadID == "" {
				results[i].Error = "thread_id is required"
				continue
			}
			updated, err := s.store.UpdateThreadStatus(mut.ThreadID, review.ThreadResolved, mut.ExpectedVersion)
			if err != nil {
				results[i].Error = err.Error()
				continue
			}
			results[i].OK = true
			results[i].Thread = updated

		case "reopen_thread":
			if mut.ThreadID == "" {
				results[i].Error = "thread_id is required"
				continue
			}
			updated, err := s.store.UpdateThreadStatus(mut.ThreadID, review.ThreadOpen, mut.ExpectedVersion)
			if err != nil {
				results[i].Error = err.Error()
				continue
			}
			results[i].OK = true
			results[i].Thread = updated

		case "delete_thread":
			if mut.ThreadID == "" {
				results[i].Error = "thread_id is required"
				continue
			}
			if err := s.store.DeleteThread(mut.ThreadID); err != nil {
				results[i].Error = err.Error()
				continue
			}
			results[i].OK = true

		default:
			results[i].Error = "unknown operation: " + mut.Op
		}
	}

	writeJSON(w, http.StatusOK, PublishAPIResponse{Results: results})
}

// --- Helpers ---

// ensureRepo gets or creates the repo from path parameters.
// Writes an error response and returns error if it fails.
func (s *Server) ensureRepo(r *http.Request) (*Repo, error) {
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	if owner == "" || repo == "" {
		return nil, errors.New("missing path params")
	}
	rp, err := s.store.GetOrCreateRepo(owner, repo)
	if err != nil {
		return nil, err
	}
	return rp, nil
}

// broadcastEvent sends an SSE event (no-op until hub is wired in spec 6).
func (s *Server) broadcastEvent(repoID, eventType string, data interface{}) {
	if s.hub != nil {
		s.hub.Broadcast(repoID, eventType, data)
	}
}
