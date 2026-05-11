package reviewd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/heiko-braun/draft/internal/review"
)

// testServer returns a Server with a real DB and a mock GitHub API that grants write access.
func testServer(t *testing.T) *Server {
	t.Helper()
	db := testDB(t)
	logger := NewLogger("error")
	srv := NewServer(db, Config{}, logger)
	// Point auth middleware at a mock GitHub that grants write access to all repos.
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"login": "testuser", "id": 1, "name": "Test User", "email": "test@example.com",
			})
		} else {
			// Any /repos/* request returns write access.
			json.NewEncoder(w).Encode(map[string]interface{}{
				"permissions": map[string]bool{"admin": false, "push": true, "pull": true},
			})
		}
	}))
	t.Cleanup(gh.Close)
	srv.auth.SetGitHubAPIBase(gh.URL)
	return srv
}

// authRequest creates a request with an auth context injected.
func authRequest(method, path string, body interface{}) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	r := httptest.NewRequest(method, path, &buf)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer test-token")
	ctx := ContextWithAuth(r.Context(), &AuthContext{
		GitHubLogin:   "testuser",
		GitHubID:      1,
		Email:         "test@example.com",
		Name:          "Test User",
		ParticipantID: "abc123",
	})
	return r.WithContext(ctx)
}

func TestAPI_ThreadLifecycle(t *testing.T) {
	srv := testServer(t)

	// Ensure participant exists for comments.
	srv.store.GetOrCreateParticipant("abc123", "Test User", "test@example.com")

	// Create thread via PUT.
	putBody := PutThreadRequest{
		Document: "docs/spec.md",
		Anchor:   review.Anchor{FileHash: "hash1", Start: 0, End: 10, Excerpt: "hello"},
	}
	req := authRequest("PUT", "/api/v1/repos/myorg/myrepo/threads/11111111-1111-1111-1111-111111111111", putBody)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create: status = %d, body = %s", w.Code, w.Body.String())
	}

	var created ThreadRow
	json.NewDecoder(w.Body).Decode(&created)
	if created.Document != "docs/spec.md" {
		t.Errorf("document = %q", created.Document)
	}
	if created.Version != 1 {
		t.Errorf("version = %d, want 1", created.Version)
	}

	// Get thread.
	req = authRequest("GET", "/api/v1/repos/myorg/myrepo/threads/"+created.ID, nil)
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("get: status = %d", w.Code)
	}

	// Add comment.
	commentBody := AddCommentAPIRequest{Body: "looks good"}
	req = authRequest("POST", "/api/v1/repos/myorg/myrepo/threads/"+created.ID+"/comments", commentBody)
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("comment: status = %d, body = %s", w.Code, w.Body.String())
	}

	// List threads.
	req = authRequest("GET", "/api/v1/repos/myorg/myrepo/threads?document=docs/spec.md", nil)
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("list: status = %d", w.Code)
	}
	var threads []ThreadRow
	json.NewDecoder(w.Body).Decode(&threads)
	if len(threads) != 1 {
		t.Fatalf("got %d threads, want 1", len(threads))
	}
	if len(threads[0].Comments) != 1 {
		t.Errorf("got %d comments, want 1", len(threads[0].Comments))
	}

	// Update thread status with optimistic concurrency.
	updateBody := PutThreadRequest{Status: "resolved"}
	req = authRequest("PUT", "/api/v1/repos/myorg/myrepo/threads/"+created.ID, updateBody)
	req.Header.Set("If-Match", "2") // version was bumped by the comment
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("resolve: status = %d, body = %s", w.Code, w.Body.String())
	}

	// Try stale update — should get 409.
	req = authRequest("PUT", "/api/v1/repos/myorg/myrepo/threads/"+created.ID, updateBody)
	req.Header.Set("If-Match", "1") // stale
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("stale update: status = %d, want 409", w.Code)
	}

	// Delete thread.
	req = authRequest("DELETE", "/api/v1/repos/myorg/myrepo/threads/"+created.ID, nil)
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("delete: status = %d", w.Code)
	}
}

func TestAPI_ReviewLifecycle(t *testing.T) {
	srv := testServer(t)

	// Create review.
	createBody := CreateReviewAPIRequest{
		Title:     "Auth spec review",
		Documents: []string{"docs/auth.md"},
		SourceRef: "abc123",
	}
	req := authRequest("POST", "/api/v1/repos/myorg/myrepo/reviews", createBody)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create: status = %d, body = %s", w.Code, w.Body.String())
	}

	var created review.Review
	json.NewDecoder(w.Body).Decode(&created)

	// Get review.
	req = authRequest("GET", "/api/v1/repos/myorg/myrepo/reviews/"+created.ID, nil)
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("get: status = %d", w.Code)
	}

	// List reviews.
	req = authRequest("GET", "/api/v1/repos/myorg/myrepo/reviews", nil)
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	var reviews []review.Review
	json.NewDecoder(w.Body).Decode(&reviews)
	if len(reviews) != 1 {
		t.Errorf("got %d reviews, want 1", len(reviews))
	}

	// Patch review.
	patchBody := PatchReviewRequest{Status: "closed"}
	req = authRequest("PATCH", "/api/v1/repos/myorg/myrepo/reviews/"+created.ID, patchBody)
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("patch: status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestAPI_Sync(t *testing.T) {
	srv := testServer(t)

	// Create a thread via store directly.
	repo, _ := srv.store.GetOrCreateRepo("myorg", "myrepo")
	thread := &review.Thread{Document: "docs/a.md", Anchor: review.Anchor{Excerpt: "x"}}
	srv.store.CreateThread(repo.ID, thread)

	// Sync with epoch — should return everything.
	syncBody := SyncRequest{Since: "2000-01-01T00:00:00Z"}
	req := authRequest("POST", "/api/v1/repos/myorg/myrepo/sync", syncBody)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("sync: status = %d", w.Code)
	}

	var resp SyncAPIResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Threads) != 1 {
		t.Errorf("got %d threads, want 1", len(resp.Threads))
	}
	if resp.ServerTime == "" {
		t.Error("server_time is empty")
	}

	// Sync with future — should return nothing.
	syncBody = SyncRequest{Since: "2099-01-01T00:00:00Z"}
	req = authRequest("POST", "/api/v1/repos/myorg/myrepo/sync", syncBody)
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Threads) != 0 {
		t.Errorf("future sync: got %d threads, want 0", len(resp.Threads))
	}
}

func TestAPI_Publish(t *testing.T) {
	srv := testServer(t)

	srv.store.GetOrCreateParticipant("abc123", "Test User", "test@example.com")

	publishBody := PublishAPIRequest{
		Mutations: []PublishMutation{
			{
				Op: "upsert_thread",
				Thread: &review.Thread{
					Document: "docs/b.md",
					Anchor:   review.Anchor{Excerpt: "text"},
				},
			},
		},
	}
	req := authRequest("POST", "/api/v1/repos/myorg/myrepo/publish", publishBody)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("publish: status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp PublishAPIResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(resp.Results))
	}
	if !resp.Results[0].OK {
		t.Errorf("mutation 0 failed: %s", resp.Results[0].Error)
	}
}
