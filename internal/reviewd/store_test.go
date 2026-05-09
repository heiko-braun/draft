package reviewd

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"

	"github.com/heiko-braun/draft/internal/review"
)

// testDB returns a connected *sql.DB with migrations applied.
// Skips the test if DATABASE_URL is not set or the DB is unreachable.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://draft:draft@localhost:5434/draft_reviews?sslmode=disable"
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("cannot open database: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("cannot connect to database: %v", err)
	}
	// Run migrations.
	if err := Migrate(db, NewLogger("error")); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Clean tables for test isolation.
	for _, tbl := range []string{"comments", "threads", "review_reviewers", "reviews", "participants", "repos"} {
		db.Exec("DELETE FROM " + tbl)
	}
	return db
}

func TestStore_RepoGetOrCreate(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	store := NewStore(db)

	r1, err := store.GetOrCreateRepo("myorg", "myrepo")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if r1.GitHubOwner != "myorg" || r1.GitHubRepo != "myrepo" {
		t.Errorf("got %s/%s, want myorg/myrepo", r1.GitHubOwner, r1.GitHubRepo)
	}

	// Idempotent.
	r2, err := store.GetOrCreateRepo("myorg", "myrepo")
	if err != nil {
		t.Fatalf("re-create: %v", err)
	}
	if r1.ID != r2.ID {
		t.Errorf("id mismatch: %s != %s", r1.ID, r2.ID)
	}
}

func TestStore_Participant(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	store := NewStore(db)

	p, err := store.GetOrCreateParticipant("p1", "Alice", "alice@example.com")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.Name != "Alice" {
		t.Errorf("name = %q, want Alice", p.Name)
	}

	p2, err := store.GetParticipant("p1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if p2.Email != "alice@example.com" {
		t.Errorf("email = %q, want alice@example.com", p2.Email)
	}

	// Not found.
	_, err = store.GetParticipant("nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestStore_ThreadCRUD(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	store := NewStore(db)

	repo, _ := store.GetOrCreateRepo("org", "repo")
	store.GetOrCreateParticipant("u1", "User", "user@test.com")

	thread := &review.Thread{
		Document: "docs/spec.md",
		Anchor: review.Anchor{
			FileHash: "abc123",
			Start:    10,
			End:      50,
			Excerpt:  "some text",
		},
	}

	created, err := store.CreateThread(repo.ID, thread)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Version != 1 {
		t.Errorf("version = %d, want 1", created.Version)
	}
	if string(created.Status) != "open" {
		t.Errorf("status = %s, want open", created.Status)
	}

	// Get.
	got, err := store.GetThread(created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Document != "docs/spec.md" {
		t.Errorf("document = %q", got.Document)
	}
	if got.Anchor.Excerpt != "some text" {
		t.Errorf("anchor excerpt = %q", got.Anchor.Excerpt)
	}

	// List by document.
	threads, err := store.ListThreadsByDocument(repo.ID, "docs/spec.md")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(threads) != 1 {
		t.Fatalf("got %d threads, want 1", len(threads))
	}

	// Update status.
	updated, err := store.UpdateThreadStatus(created.ID, review.ThreadResolved, 1)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if updated.Version != 2 {
		t.Errorf("version = %d, want 2", updated.Version)
	}
	if string(updated.Status) != "resolved" {
		t.Errorf("status = %s, want resolved", updated.Status)
	}

	// Optimistic concurrency conflict.
	_, err = store.UpdateThreadStatus(created.ID, review.ThreadOpen, 1) // stale version
	if err != ErrVersionConflict {
		t.Errorf("expected ErrVersionConflict, got %v", err)
	}

	// Delete.
	if err := store.DeleteThread(created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = store.GetThread(created.ID)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestStore_Comments(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	store := NewStore(db)

	repo, _ := store.GetOrCreateRepo("org", "repo")
	store.GetOrCreateParticipant("u1", "Alice", "alice@test.com")

	thread := &review.Thread{Document: "docs/a.md", Anchor: review.Anchor{Excerpt: "x"}}
	created, err := store.CreateThread(repo.ID, thread)
	if err != nil {
		t.Fatalf("create thread: %v", err)
	}

	c1, err := store.AddComment(created.ID, "u1", "first comment")
	if err != nil {
		t.Fatalf("add comment: %v", err)
	}
	if c1.Body != "first comment" {
		t.Errorf("body = %q", c1.Body)
	}

	c2, err := store.AddComment(created.ID, "u1", "second comment")
	if err != nil {
		t.Fatalf("add comment 2: %v", err)
	}

	comments, err := store.ListComments(created.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("got %d comments, want 2", len(comments))
	}
	if comments[0].ID != c1.ID || comments[1].ID != c2.ID {
		t.Error("comments not ordered correctly")
	}
}

func TestStore_CommentAutoReopens(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	store := NewStore(db)

	repo, _ := store.GetOrCreateRepo("org", "repo")
	store.GetOrCreateParticipant("u1", "User", "user@test.com")

	thread := &review.Thread{Document: "docs/a.md", Anchor: review.Anchor{Excerpt: "x"}}
	created, _ := store.CreateThread(repo.ID, thread)
	store.UpdateThreadStatus(created.ID, review.ThreadResolved, 1)

	// Comment on resolved thread should auto-reopen.
	store.AddComment(created.ID, "u1", "wait, one more thing")

	got, _ := store.GetThread(created.ID)
	if string(got.Status) != "open" {
		t.Errorf("status = %s, want open (auto-reopen)", got.Status)
	}
}

func TestStore_ReviewCRUD(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	store := NewStore(db)

	repo, _ := store.GetOrCreateRepo("org", "repo")
	store.GetOrCreateParticipant("u1", "User", "user@test.com")

	r, err := store.CreateReview(repo.ID, "My Review", []string{"docs/a.md"}, "abc123")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if r.Title != "My Review" {
		t.Errorf("title = %q", r.Title)
	}

	got, err := store.GetReview(r.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got.Documents) != 1 || got.Documents[0] != "docs/a.md" {
		t.Errorf("documents = %v", got.Documents)
	}

	// Add reviewer.
	if err := store.AddReviewer(r.ID, "u1"); err != nil {
		t.Fatalf("add reviewer: %v", err)
	}
	got, _ = store.GetReview(r.ID)
	if len(got.Reviewers) != 1 {
		t.Fatalf("got %d reviewers, want 1", len(got.Reviewers))
	}

	// Remove reviewer.
	if err := store.RemoveReviewer(r.ID, "u1"); err != nil {
		t.Fatalf("remove reviewer: %v", err)
	}
	got, _ = store.GetReview(r.ID)
	if len(got.Reviewers) != 0 {
		t.Errorf("got %d reviewers after remove", len(got.Reviewers))
	}

	// Update status.
	if err := store.UpdateReviewStatus(r.ID, review.ReviewClosed); err != nil {
		t.Fatalf("update status: %v", err)
	}
	got, _ = store.GetReview(r.ID)
	if string(got.Status) != "closed" {
		t.Errorf("status = %s, want closed", got.Status)
	}

	// List.
	reviews, err := store.ListReviewsByRepo(repo.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(reviews) != 1 {
		t.Errorf("got %d reviews, want 1", len(reviews))
	}
}
