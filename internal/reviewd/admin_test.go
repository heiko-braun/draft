package reviewd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminOnly_AllowsAdminEmail(t *testing.T) {
	mw := AdminOnly([]string{"admin@example.com"})

	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/admin", nil)
	ctx := ContextWithAuth(req.Context(), &AuthContext{Email: "admin@example.com"})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestAdminOnly_BlocksNonAdmin(t *testing.T) {
	mw := AdminOnly([]string{"admin@example.com"})

	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	})

	req := httptest.NewRequest("GET", "/admin", nil)
	ctx := ContextWithAuth(req.Context(), &AuthContext{Email: "user@example.com"})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestAdminOnly_BlocksUnauthenticated(t *testing.T) {
	mw := AdminOnly([]string{"admin@example.com"})

	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	})

	req := httptest.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestAdminOnly_CaseInsensitive(t *testing.T) {
	mw := AdminOnly([]string{"Admin@Example.COM"})

	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/admin", nil)
	ctx := ContextWithAuth(req.Context(), &AuthContext{Email: "admin@example.com"})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestHandleAdmin_ReturnsHTML(t *testing.T) {
	if testing.Short() {
		t.Skip("requires database")
	}

	db := testDB(t)
	logger := NewLogger("error")
	cfg := Config{AdminEmails: []string{"admin@test.com"}}
	s := &Server{
		db:     db,
		store:  NewStore(db),
		logger: logger,
		config: cfg,
	}

	// Seed some data.
	db.Exec("INSERT INTO repos (github_owner, github_repo) VALUES ('org', 'repo1')")
	db.Exec("INSERT INTO participants (id, name, email) VALUES ('p1', 'Alice', 'alice@test.com')")
	db.Exec("INSERT INTO participants (id, name, email) VALUES ('p2', 'Bob', 'bob@test.com')")

	req := httptest.NewRequest("GET", "/admin", nil)
	ctx := ContextWithAuth(context.Background(), &AuthContext{Email: "admin@test.com"})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	s.handleAdmin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("response is not HTML")
	}
	// Should contain the participant count of 2.
	if !strings.Contains(body, ">2<") {
		t.Errorf("expected participant count 2 in body")
	}
	// Should contain repo count of 1.
	if !strings.Contains(body, ">1<") {
		t.Errorf("expected repo count 1 in body")
	}
}
