package review

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewUUID(t *testing.T) {
	id1, err := newUUID()
	if err != nil {
		t.Fatal(err)
	}
	id2, err := newUUID()
	if err != nil {
		t.Fatal(err)
	}

	if id1 == id2 {
		t.Error("newUUID generated duplicate IDs")
	}

	// Check format: 8-4-4-4-12 hex chars.
	if len(id1) != 36 {
		t.Errorf("UUID length = %d, want 36", len(id1))
	}
	// Check version nibble (position 14 should be '4').
	if id1[14] != '4' {
		t.Errorf("UUID version nibble = %c, want '4'", id1[14])
	}
}

func TestCreateReview_And_GetReview(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	r, err := store.CreateReview("Test Review", []string{"docs/spec.md"}, "abc123")
	if err != nil {
		t.Fatal(err)
	}

	if r.ID == "" {
		t.Error("Review ID should not be empty")
	}
	if r.Title != "Test Review" {
		t.Errorf("Title = %q, want %q", r.Title, "Test Review")
	}
	if r.Status != ReviewOpen {
		t.Errorf("Status = %q, want %q", r.Status, ReviewOpen)
	}
	if len(r.Documents) != 1 || r.Documents[0] != "docs/spec.md" {
		t.Errorf("Documents = %v, want [docs/spec.md]", r.Documents)
	}
	if r.SourceRef != "abc123" {
		t.Errorf("SourceRef = %q, want %q", r.SourceRef, "abc123")
	}
	if r.CreatedAt == "" {
		t.Error("CreatedAt should not be empty")
	}

	// Read it back.
	got, err := store.GetReview(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != r.ID {
		t.Errorf("GetReview ID = %q, want %q", got.ID, r.ID)
	}
	if got.Title != r.Title {
		t.Errorf("GetReview Title = %q, want %q", got.Title, r.Title)
	}
}

func TestGetReview_NotFound(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	_, err := store.GetReview("nonexistent")
	if err == nil {
		t.Error("GetReview should fail for nonexistent ID")
	}
}

func TestUpdateReviewStatus(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	r, err := store.CreateReview("Status Test", []string{"doc.md"}, "ref1")
	if err != nil {
		t.Fatal(err)
	}

	// Backdate the original timestamps so the update is guaranteed to differ.
	r.CreatedAt = "2024-01-01T00:00:00Z"
	r.UpdatedAt = "2024-01-01T00:00:00Z"
	if err := writeJSON(store.reviewPath(r.ID), r); err != nil {
		t.Fatal(err)
	}

	if err := store.UpdateReviewStatus(r.ID, ReviewClosed); err != nil {
		t.Fatal(err)
	}

	updated, err := store.GetReview(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != ReviewClosed {
		t.Errorf("Status = %q, want %q", updated.Status, ReviewClosed)
	}
	if updated.UpdatedAt == "2024-01-01T00:00:00Z" {
		t.Error("UpdatedAt should have changed from the backdated value")
	}
}

func TestListReviews(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	_, err := store.CreateReview("Review 1", []string{"a.md"}, "ref1")
	if err != nil {
		t.Fatal(err)
	}
	r2, err := store.CreateReview("Review 2", []string{"b.md"}, "ref2")
	if err != nil {
		t.Fatal(err)
	}

	// Close one.
	if err := store.UpdateReviewStatus(r2.ID, ReviewMerged); err != nil {
		t.Fatal(err)
	}

	all, err := store.ListReviews()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("ListReviews count = %d, want 2", len(all))
	}

	open, err := store.ListOpenReviews()
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 1 {
		t.Fatalf("ListOpenReviews count = %d, want 1", len(open))
	}
	if open[0].Title != "Review 1" {
		t.Errorf("open review title = %q, want %q", open[0].Title, "Review 1")
	}
}

func TestListReviews_EmptyDir(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	reviews, err := store.ListReviews()
	if err != nil {
		t.Fatal(err)
	}
	if len(reviews) != 0 {
		t.Errorf("ListReviews on empty dir = %d, want 0", len(reviews))
	}
}

func TestAddReviewer_And_RemoveReviewer(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	r, err := store.CreateReview("Reviewer Test", []string{"doc.md"}, "ref")
	if err != nil {
		t.Fatal(err)
	}

	if err := store.AddReviewer(r.ID, "user-123"); err != nil {
		t.Fatal(err)
	}

	// Adding same reviewer again is a no-op.
	if err := store.AddReviewer(r.ID, "user-123"); err != nil {
		t.Fatal(err)
	}

	got, err := store.GetReview(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Reviewers) != 1 {
		t.Fatalf("Reviewers count = %d, want 1", len(got.Reviewers))
	}
	if got.Reviewers[0].ParticipantID != "user-123" {
		t.Errorf("Reviewer ID = %q, want %q", got.Reviewers[0].ParticipantID, "user-123")
	}
	if got.Reviewers[0].Status != "pending" {
		t.Errorf("Reviewer Status = %q, want %q", got.Reviewers[0].Status, "pending")
	}

	// Remove reviewer.
	if err := store.RemoveReviewer(r.ID, "user-123"); err != nil {
		t.Fatal(err)
	}
	got, err = store.GetReview(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Reviewers) != 0 {
		t.Errorf("Reviewers count after remove = %d, want 0", len(got.Reviewers))
	}
}

func TestCreateThread_And_GetThread(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	anchor := Anchor{
		FileHash: "hash123",
		Start:    0,
		End:      12,
		Excerpt:  "test excerpt",
	}

	thread, err := store.CreateThread("review-1", "docs/spec.md", anchor)
	if err != nil {
		t.Fatal(err)
	}

	if thread.ID == "" {
		t.Error("Thread ID should not be empty")
	}
	if thread.Document != "docs/spec.md" {
		t.Errorf("Document = %q, want %q", thread.Document, "docs/spec.md")
	}
	if thread.ReviewID != "review-1" {
		t.Errorf("ReviewID = %q, want %q", thread.ReviewID, "review-1")
	}
	if thread.Status != ThreadOpen {
		t.Errorf("Status = %q, want %q", thread.Status, ThreadOpen)
	}
	if thread.CreatedAt == "" {
		t.Error("CreatedAt should not be empty")
	}

	// Read it back.
	got, err := store.GetThread("docs/spec.md", thread.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != thread.ID {
		t.Errorf("GetThread ID = %q, want %q", got.ID, thread.ID)
	}
	if got.Anchor.Excerpt != "test excerpt" {
		t.Errorf("Anchor Excerpt = %q, want %q", got.Anchor.Excerpt, "test excerpt")
	}
}

func TestAddComment(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	anchor := Anchor{FileHash: "hashA", Start: 0, End: 5, Excerpt: "hello"}
	thread, err := store.CreateThread("rev-1", "doc.md", anchor)
	if err != nil {
		t.Fatal(err)
	}

	c, err := store.AddComment("doc.md", thread.ID, "user-1", "Hello!")
	if err != nil {
		t.Fatal(err)
	}
	if c.ID == "" {
		t.Error("Comment ID should not be empty")
	}
	if c.Author != "user-1" {
		t.Errorf("Author = %q, want %q", c.Author, "user-1")
	}
	if c.Body != "Hello!" {
		t.Errorf("Body = %q, want %q", c.Body, "Hello!")
	}

	// Add a second comment.
	c2, err := store.AddComment("doc.md", thread.ID, "user-2", "World!")
	if err != nil {
		t.Fatal(err)
	}

	// Verify both comments are stored.
	got, err := store.GetThread("doc.md", thread.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Comments) != 2 {
		t.Fatalf("Comments count = %d, want 2", len(got.Comments))
	}
	if got.Comments[0].ID != c.ID {
		t.Errorf("First comment ID = %q, want %q", got.Comments[0].ID, c.ID)
	}
	if got.Comments[1].ID != c2.ID {
		t.Errorf("Second comment ID = %q, want %q", got.Comments[1].ID, c2.ID)
	}
}

func TestResolveThread_And_ReopenThread(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	anchor := Anchor{FileHash: "hashB", Start: 10, End: 20, Excerpt: "some content"}
	thread, err := store.CreateThread("rev-1", "doc.md", anchor)
	if err != nil {
		t.Fatal(err)
	}

	// Resolve.
	if err := store.ResolveThread("doc.md", thread.ID); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetThread("doc.md", thread.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != ThreadResolved {
		t.Errorf("Status = %q, want %q", got.Status, ThreadResolved)
	}

	// Reopen.
	if err := store.ReopenThread("doc.md", thread.ID); err != nil {
		t.Fatal(err)
	}
	got, err = store.GetThread("doc.md", thread.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != ThreadOpen {
		t.Errorf("Status = %q, want %q", got.Status, ThreadOpen)
	}
}

func TestListThreadsByDocument(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	anchor := Anchor{FileHash: "hashX", Start: 0, End: 8, Excerpt: "anchor X"}

	_, err := store.CreateThread("rev-1", "docs/a.md", anchor)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.CreateThread("rev-1", "docs/a.md", anchor)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.CreateThread("rev-1", "docs/b.md", anchor)
	if err != nil {
		t.Fatal(err)
	}

	threads, err := store.ListThreadsByDocument("docs/a.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 2 {
		t.Errorf("threads for a.md = %d, want 2", len(threads))
	}

	threads, err = store.ListThreadsByDocument("docs/b.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 1 {
		t.Errorf("threads for b.md = %d, want 1", len(threads))
	}

	threads, err = store.ListThreadsByDocument("nonexistent.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 0 {
		t.Errorf("threads for nonexistent = %d, want 0", len(threads))
	}
}

func TestListAllThreads(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	anchor := Anchor{FileHash: "hashY", Start: 0, End: 8, Excerpt: "anchor Y"}

	_, err := store.CreateThread("rev-1", "docs/a.md", anchor)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.CreateThread("rev-1", "docs/b.md", anchor)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.CreateThread("rev-2", "specs/s.md", anchor)
	if err != nil {
		t.Fatal(err)
	}

	all, err := store.ListAllThreads()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("ListAllThreads = %d, want 3", len(all))
	}
}

func TestListAllThreads_EmptyDir(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	threads, err := store.ListAllThreads()
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 0 {
		t.Errorf("ListAllThreads on empty = %d, want 0", len(threads))
	}
}

func TestUpdateThreadAnchor(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	anchor := Anchor{
		FileHash: "oldhash",
		Start:    0,
		End:      10,
		Excerpt:  "original text",
	}
	thread, err := store.CreateThread("rev-1", "doc.md", anchor)
	if err != nil {
		t.Fatal(err)
	}

	newAnchor := Anchor{
		FileHash: "newhash",
		Start:    30,
		End:      45,
		Excerpt:  "updated text",
	}
	if err := store.UpdateThreadAnchor("doc.md", thread.ID, newAnchor); err != nil {
		t.Fatal(err)
	}

	got, err := store.GetThread("doc.md", thread.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Anchor.FileHash != "newhash" {
		t.Errorf("Anchor FileHash = %q, want %q", got.Anchor.FileHash, "newhash")
	}
	if got.Anchor.Start != 30 {
		t.Errorf("Anchor Start = %d, want 30", got.Anchor.Start)
	}
}

func TestEnsureParticipant(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	p, err := store.EnsureParticipant("user-abc", "Alice", "alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "user-abc" {
		t.Errorf("ID = %q, want %q", p.ID, "user-abc")
	}
	if p.Name != "Alice" {
		t.Errorf("Name = %q, want %q", p.Name, "Alice")
	}
	if p.Email != "alice@example.com" {
		t.Errorf("Email = %q, want %q", p.Email, "alice@example.com")
	}

	// Calling again returns existing.
	p2, err := store.EnsureParticipant("user-abc", "Alice Updated", "alice-new@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if p2.Name != "Alice" {
		t.Errorf("existing participant Name = %q, want %q (should not update)", p2.Name, "Alice")
	}
}

func TestGetParticipant(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	_, err := store.EnsureParticipant("p-1", "Bob", "bob@example.com")
	if err != nil {
		t.Fatal(err)
	}

	got, err := store.GetParticipant("p-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Bob" {
		t.Errorf("Name = %q, want %q", got.Name, "Bob")
	}

	_, err = store.GetParticipant("nonexistent")
	if err == nil {
		t.Error("GetParticipant should fail for nonexistent ID")
	}
}

func TestParticipantIDFromEmail(t *testing.T) {
	id1 := participantIDFromEmail("user@example.com")
	id2 := participantIDFromEmail("USER@Example.com")
	id3 := participantIDFromEmail("  user@example.com  ")

	// Should be case-insensitive and trim whitespace.
	if id1 != id2 {
		t.Errorf("IDs should match for case-insensitive emails: %q vs %q", id1, id2)
	}
	if id1 != id3 {
		t.Errorf("IDs should match after whitespace trimming: %q vs %q", id1, id3)
	}

	// Should be 16 hex chars.
	if len(id1) != 16 {
		t.Errorf("participant ID length = %d, want 16", len(id1))
	}
}

func TestMergeThreads_NonOverlappingComments(t *testing.T) {
	ours := &Thread{
		ID:        "t-1",
		Document:  "doc.md",
		Status:    ThreadOpen,
		UpdatedAt: "2024-01-01T10:00:00Z",
		Comments: []Comment{
			{ID: "c-1", Author: "alice", Body: "First", CreatedAt: "2024-01-01T09:00:00Z"},
		},
	}
	theirs := &Thread{
		ID:        "t-1",
		Document:  "doc.md",
		Status:    ThreadOpen,
		UpdatedAt: "2024-01-01T11:00:00Z",
		Comments: []Comment{
			{ID: "c-2", Author: "bob", Body: "Second", CreatedAt: "2024-01-01T10:30:00Z"},
		},
	}

	merged := MergeThreads(ours, theirs)

	if len(merged.Comments) != 2 {
		t.Fatalf("merged comments = %d, want 2", len(merged.Comments))
	}
	// Should be ordered by CreatedAt.
	if merged.Comments[0].ID != "c-1" {
		t.Errorf("first comment = %q, want c-1", merged.Comments[0].ID)
	}
	if merged.Comments[1].ID != "c-2" {
		t.Errorf("second comment = %q, want c-2", merged.Comments[1].ID)
	}
}

func TestMergeThreads_DuplicateComments(t *testing.T) {
	ours := &Thread{
		ID:        "t-1",
		Document:  "doc.md",
		Status:    ThreadOpen,
		UpdatedAt: "2024-01-01T10:00:00Z",
		Comments: []Comment{
			{ID: "c-1", Author: "alice", Body: "Original", CreatedAt: "2024-01-01T09:00:00Z"},
		},
	}
	theirs := &Thread{
		ID:        "t-1",
		Document:  "doc.md",
		Status:    ThreadOpen,
		UpdatedAt: "2024-01-01T10:00:00Z",
		Comments: []Comment{
			{ID: "c-1", Author: "alice", Body: "Edited", CreatedAt: "2024-01-01T09:00:00Z", UpdatedAt: "2024-01-01T09:30:00Z"},
		},
	}

	merged := MergeThreads(ours, theirs)

	if len(merged.Comments) != 1 {
		t.Fatalf("merged comments = %d, want 1 (deduped)", len(merged.Comments))
	}
	// Should pick the later version (theirs has UpdatedAt).
	if merged.Comments[0].Body != "Edited" {
		t.Errorf("comment body = %q, want %q (should pick later version)", merged.Comments[0].Body, "Edited")
	}
}

func TestMergeThreads_ConflictingStatus_PicksLatest(t *testing.T) {
	ours := &Thread{
		ID:        "t-1",
		Document:  "doc.md",
		Status:    ThreadResolved,
		UpdatedAt: "2024-01-01T10:00:00Z",
	}
	theirs := &Thread{
		ID:        "t-1",
		Document:  "doc.md",
		Status:    ThreadWontFix,
		UpdatedAt: "2024-01-01T12:00:00Z",
	}

	merged := MergeThreads(ours, theirs)

	if merged.Status != ThreadWontFix {
		t.Errorf("Status = %q, want %q (theirs is later)", merged.Status, ThreadWontFix)
	}
	if merged.UpdatedAt != "2024-01-01T12:00:00Z" {
		t.Errorf("UpdatedAt = %q, want %q", merged.UpdatedAt, "2024-01-01T12:00:00Z")
	}
}

func TestMergeThreads_OursStatusWins_WhenLater(t *testing.T) {
	ours := &Thread{
		ID:        "t-1",
		Document:  "doc.md",
		Status:    ThreadResolved,
		UpdatedAt: "2024-01-01T15:00:00Z",
	}
	theirs := &Thread{
		ID:        "t-1",
		Document:  "doc.md",
		Status:    ThreadOpen,
		UpdatedAt: "2024-01-01T10:00:00Z",
	}

	merged := MergeThreads(ours, theirs)

	if merged.Status != ThreadResolved {
		t.Errorf("Status = %q, want %q (ours is later)", merged.Status, ThreadResolved)
	}
}

func TestReview_JSONRoundTrip(t *testing.T) {
	r := Review{
		ID:        "rev-123",
		Title:     "Test Review",
		Status:    ReviewOpen,
		Documents: []string{"docs/a.md", "docs/b.md"},
		Reviewers: []ReviewerEntry{
			{ParticipantID: "user-1", Status: "pending"},
		},
		SourceRef: "abcdef",
		CreatedAt: "2024-01-01T00:00:00Z",
		UpdatedAt: "2024-01-01T00:00:00Z",
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	var got Review
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}

	if got.ID != r.ID {
		t.Errorf("ID = %q, want %q", got.ID, r.ID)
	}
	if got.Status != r.Status {
		t.Errorf("Status = %q, want %q", got.Status, r.Status)
	}
	if len(got.Documents) != 2 {
		t.Errorf("Documents count = %d, want 2", len(got.Documents))
	}
	if len(got.Reviewers) != 1 {
		t.Errorf("Reviewers count = %d, want 1", len(got.Reviewers))
	}
}

func TestThread_JSONRoundTrip_MinimalForAnchor(t *testing.T) {
	// The anchor system uses Thread with only ID, Document, Anchor.
	// Verify that omitempty fields don't break the minimal case.
	thread := Thread{
		ID:       "t-1",
		Document: "doc.md",
		Anchor: Anchor{
			FileHash: "hash",
			Start:    0,
			End:      10,
			Excerpt:  "goal text",
		},
	}

	data, err := json.Marshal(thread)
	if err != nil {
		t.Fatal(err)
	}

	var got Thread
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}

	if got.ID != "t-1" {
		t.Errorf("ID = %q, want %q", got.ID, "t-1")
	}
	if got.Document != "doc.md" {
		t.Errorf("Document = %q, want %q", got.Document, "doc.md")
	}
	if got.ReviewID != "" {
		t.Errorf("ReviewID = %q, want empty (omitempty)", got.ReviewID)
	}
	if got.Status != "" {
		t.Errorf("Status = %q, want empty (omitempty)", got.Status)
	}
	if got.Comments != nil {
		t.Errorf("Comments = %v, want nil (omitempty)", got.Comments)
	}
}

func TestThread_JSONRoundTrip_Full(t *testing.T) {
	thread := Thread{
		ID:       "t-full",
		Document: "specs/feature.md",
		Anchor: Anchor{
			FileHash: "abc",
			Start:    5,
			End:      20,
			Excerpt:  "test excerpt",
		},
		ReviewID:  "rev-1",
		Status:    ThreadResolved,
		CreatedAt: "2024-01-01T00:00:00Z",
		UpdatedAt: "2024-01-02T00:00:00Z",
		Comments: []Comment{
			{ID: "c-1", Author: "user-1", Body: "Nice!", CreatedAt: "2024-01-01T01:00:00Z"},
		},
	}

	data, err := json.MarshalIndent(thread, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	var got Thread
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}

	if got.ReviewID != "rev-1" {
		t.Errorf("ReviewID = %q, want %q", got.ReviewID, "rev-1")
	}
	if got.Status != ThreadResolved {
		t.Errorf("Status = %q, want %q", got.Status, ThreadResolved)
	}
	if len(got.Comments) != 1 {
		t.Fatalf("Comments count = %d, want 1", len(got.Comments))
	}
	if got.Comments[0].Body != "Nice!" {
		t.Errorf("Comment body = %q, want %q", got.Comments[0].Body, "Nice!")
	}
}

func TestComment_JSONRoundTrip(t *testing.T) {
	c := Comment{
		ID:        "c-1",
		Author:    "user-1",
		Body:      "This looks good.",
		CreatedAt: "2024-01-01T00:00:00Z",
		UpdatedAt: "2024-01-02T00:00:00Z",
	}

	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}

	var got Comment
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}

	if got.ID != c.ID {
		t.Errorf("ID = %q, want %q", got.ID, c.ID)
	}
	if got.Body != c.Body {
		t.Errorf("Body = %q, want %q", got.Body, c.Body)
	}
	if got.UpdatedAt != c.UpdatedAt {
		t.Errorf("UpdatedAt = %q, want %q", got.UpdatedAt, c.UpdatedAt)
	}
}

func TestParticipant_JSONRoundTrip(t *testing.T) {
	p := Participant{
		ID:    "p-1",
		Name:  "Alice",
		Email: "alice@example.com",
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}

	var got Participant
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}

	if got.ID != p.ID {
		t.Errorf("ID = %q, want %q", got.ID, p.ID)
	}
	if got.Name != p.Name {
		t.Errorf("Name = %q, want %q", got.Name, p.Name)
	}
	if got.Email != p.Email {
		t.Errorf("Email = %q, want %q", got.Email, p.Email)
	}
}

func TestStore_FileLayout(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	// Create a review.
	r, err := store.CreateReview("Layout Test", []string{"spec.md"}, "ref")
	if err != nil {
		t.Fatal(err)
	}

	// Verify review file location.
	reviewFile := filepath.Join(root, "reviews", r.ID+".json")
	if _, err := os.Stat(reviewFile); err != nil {
		t.Errorf("review file not found at expected path: %s", reviewFile)
	}

	// Create a thread.
	anchor := Anchor{FileHash: "hashTest", Start: 0, End: 15, Excerpt: "test section"}
	thread, err := store.CreateThread(r.ID, "docs/nested/doc.md", anchor)
	if err != nil {
		t.Fatal(err)
	}

	// Verify thread file location: threads/{document-path}/{thread-id}.json
	threadFile := filepath.Join(root, "threads", "docs", "nested", "doc.md", thread.ID+".json")
	if _, err := os.Stat(threadFile); err != nil {
		t.Errorf("thread file not found at expected path: %s", threadFile)
	}

	// Create a participant.
	_, err = store.EnsureParticipant("user-hash-abc", "Charlie", "charlie@example.com")
	if err != nil {
		t.Fatal(err)
	}

	// Verify participant file location.
	participantFile := filepath.Join(root, "participants", "user-hash-abc.json")
	if _, err := os.Stat(participantFile); err != nil {
		t.Errorf("participant file not found at expected path: %s", participantFile)
	}
}

func TestStaleApprovalDetection(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	r, err := store.CreateReview("Stale Test", []string{"doc.md"}, "commit-abc")
	if err != nil {
		t.Fatal(err)
	}

	// Add a reviewer with an approval at a specific source ref.
	if err := store.AddReviewer(r.ID, "reviewer-1"); err != nil {
		t.Fatal(err)
	}

	// Simulate an approval by directly modifying the review.
	got, err := store.GetReview(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	got.Reviewers[0].Status = "approved"
	got.Reviewers[0].ApprovedAt = "2024-01-01T10:00:00Z"
	got.Reviewers[0].ApprovalSourceRef = "commit-abc"
	if err := writeJSON(store.reviewPath(r.ID), got); err != nil {
		t.Fatal(err)
	}

	// Read back and check: approval is fresh when source_ref matches.
	fresh, err := store.GetReview(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Reviewers[0].ApprovalSourceRef != fresh.SourceRef {
		t.Error("approval should be fresh when source_refs match")
	}

	// Update SourceRef to simulate a new document commit — approval becomes stale.
	fresh.SourceRef = "commit-def"
	if err := writeJSON(store.reviewPath(r.ID), fresh); err != nil {
		t.Fatal(err)
	}

	stale, err := store.GetReview(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stale.Reviewers[0].ApprovalSourceRef == stale.SourceRef {
		t.Error("approval should be stale when source_refs differ")
	}
}
