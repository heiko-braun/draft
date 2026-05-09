package reviewd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func mockGitHubAPI() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		switch token {
		case "Bearer valid-token":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"login": "testuser",
				"id":    12345,
				"name":  "Test User",
				"email": "test@example.com",
			})
		case "Bearer no-email-token":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"login": "nomail",
				"id":    99,
				"name":  "No Mail",
				"email": "",
			})
		default:
			w.WriteHeader(401)
		}
	})

	mux.HandleFunc("/repos/myorg/myrepo", func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token != "Bearer valid-token" {
			w.WriteHeader(404)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"permissions": map[string]bool{
				"admin": false,
				"push":  true,
				"pull":  true,
			},
		})
	})

	mux.HandleFunc("/repos/myorg/readonly", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"permissions": map[string]bool{
				"admin": false,
				"push":  false,
				"pull":  true,
			},
		})
	})

	return httptest.NewServer(mux)
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	gh := mockGitHubAPI()
	defer gh.Close()

	am := NewAuthMiddleware(NewLogger("error"))
	am.SetGitHubAPIBase(gh.URL)

	var gotUser *AuthContext
	handler := am.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFromContext(r.Context())
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/api/v1/repos/org/repo/threads", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if gotUser == nil {
		t.Fatal("user not set in context")
	}
	if gotUser.GitHubLogin != "testuser" {
		t.Errorf("login = %q, want testuser", gotUser.GitHubLogin)
	}
	if gotUser.Email != "test@example.com" {
		t.Errorf("email = %q", gotUser.Email)
	}
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	am := NewAuthMiddleware(NewLogger("error"))

	handler := am.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	gh := mockGitHubAPI()
	defer gh.Close()

	am := NewAuthMiddleware(NewLogger("error"))
	am.SetGitHubAPIBase(gh.URL)

	handler := am.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuthMiddleware_HealthBypass(t *testing.T) {
	am := NewAuthMiddleware(NewLogger("error"))

	called := false
	handler := am.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 || !called {
		t.Errorf("healthz should bypass auth: status=%d called=%v", w.Code, called)
	}
}

func TestAuthMiddleware_CachesToken(t *testing.T) {
	callCount := 0
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user" {
			callCount++
			json.NewEncoder(w).Encode(map[string]interface{}{
				"login": "cached", "id": 1, "name": "C", "email": "c@test.com",
			})
		}
	}))
	defer gh.Close()

	am := NewAuthMiddleware(NewLogger("error"))
	am.SetGitHubAPIBase(gh.URL)

	handler := am.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.Header.Set("Authorization", "Bearer cache-test")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	if callCount != 1 {
		t.Errorf("GitHub API called %d times, want 1 (cached)", callCount)
	}
}

func TestAuthMiddleware_RepoAccessWrite(t *testing.T) {
	gh := mockGitHubAPI()
	defer gh.Close()

	am := NewAuthMiddleware(NewLogger("error"))
	am.SetGitHubAPIBase(gh.URL)

	// First authenticate the token.
	handler := am.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check repo access inside the handler.
		am.RequireRepoAccess(AccessWrite, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
		})(w, r)
	}))

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/repos/{owner}/{repo}/threads", handler)

	req := httptest.NewRequest("GET", "/api/v1/repos/myorg/myrepo/threads", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}
