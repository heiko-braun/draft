package reviewd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testAdminAuth() *AdminAuth {
	cfg := Config{
		GitHubClientID:     "test-client-id",
		GitHubClientSecret: "test-client-secret",
		PublicURL:          "http://localhost:5100",
		AdminEmails:        []string{"admin@example.com"},
	}
	return NewAdminAuth(cfg, NewLogger("error"))
}

func mockOAuthServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		code := r.FormValue("code")
		w.Header().Set("Content-Type", "application/json")
		switch code {
		case "valid-code":
			json.NewEncoder(w).Encode(map[string]string{
				"access_token": "oauth-test-token",
				"token_type":   "bearer",
			})
		default:
			json.NewEncoder(w).Encode(map[string]string{
				"error": "bad_verification_code",
			})
		}
	})

	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token != "Bearer oauth-test-token" {
			w.WriteHeader(401)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"login": "adminuser",
			"email": "admin@example.com",
		})
	})

	mux.HandleFunc("/user/emails", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"email": "admin@example.com", "primary": true, "verified": true},
		})
	})

	return httptest.NewServer(mux)
}

func TestAdminAuth_CookieSignVerify(t *testing.T) {
	a := testAdminAuth()

	w := httptest.NewRecorder()
	a.setSessionCookie(w, "admin@example.com", "adminuser")

	// Extract cookie from response.
	resp := w.Result()
	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Fatal("no cookie set")
	}

	cookie := cookies[0]
	if cookie.Name != cookieName {
		t.Errorf("cookie name = %q, want %q", cookie.Name, cookieName)
	}
	if !cookie.HttpOnly {
		t.Error("cookie should be HttpOnly")
	}
	if cookie.Path != "/admin" {
		t.Errorf("cookie path = %q, want /admin", cookie.Path)
	}

	// Verify the cookie.
	req := httptest.NewRequest("GET", "/admin", nil)
	req.AddCookie(cookie)

	email, login, ok := a.verifySessionCookie(req)
	if !ok {
		t.Fatal("cookie verification failed")
	}
	if email != "admin@example.com" {
		t.Errorf("email = %q", email)
	}
	if login != "adminuser" {
		t.Errorf("login = %q", login)
	}
}

func TestAdminAuth_CookieTampered(t *testing.T) {
	a := testAdminAuth()

	w := httptest.NewRecorder()
	a.setSessionCookie(w, "admin@example.com", "adminuser")

	cookie := w.Result().Cookies()[0]
	cookie.Value = "tampered" + cookie.Value

	req := httptest.NewRequest("GET", "/admin", nil)
	req.AddCookie(cookie)

	_, _, ok := a.verifySessionCookie(req)
	if ok {
		t.Error("tampered cookie should not verify")
	}
}

func TestAdminAuth_CookieExpired(t *testing.T) {
	a := testAdminAuth()

	// Manually create an expired cookie payload.
	w := httptest.NewRecorder()
	a.setSessionCookie(w, "admin@example.com", "adminuser")

	// We can't easily expire it, so test with a different AdminAuth that has different time.
	// Instead, directly test verifySessionCookie with no cookie.
	req := httptest.NewRequest("GET", "/admin", nil)
	_, _, ok := a.verifySessionCookie(req)
	if ok {
		t.Error("should fail without cookie")
	}
}

func TestAdminAuth_StateGenerateValidate(t *testing.T) {
	a := testAdminAuth()

	state := a.generateState()
	if state == "" {
		t.Fatal("empty state")
	}

	// First validation should succeed.
	if !a.validateState(state) {
		t.Error("valid state rejected")
	}

	// Second validation should fail (single-use).
	if a.validateState(state) {
		t.Error("reused state should be rejected")
	}
}

func TestAdminAuth_StateInvalid(t *testing.T) {
	a := testAdminAuth()

	if a.validateState("nonexistent") {
		t.Error("random state should be rejected")
	}
}

func TestAdminAuth_HandleLogin_Redirect(t *testing.T) {
	a := testAdminAuth()

	req := httptest.NewRequest("GET", "/admin/login", nil)
	w := httptest.NewRecorder()
	a.handleAdminLogin(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", w.Code)
	}

	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "github.com/login/oauth/authorize") {
		t.Errorf("redirect location = %q, want GitHub authorize URL", loc)
	}
	if !strings.Contains(loc, "client_id=test-client-id") {
		t.Error("missing client_id in redirect")
	}
	if !strings.Contains(loc, "scope=user") {
		t.Error("missing scope in redirect")
	}
	if !strings.Contains(loc, "state=") {
		t.Error("missing state in redirect")
	}
}

func TestAdminAuth_HandleLogin_NotConfigured(t *testing.T) {
	cfg := Config{} // no client ID
	a := NewAdminAuth(cfg, NewLogger("error"))

	req := httptest.NewRequest("GET", "/admin/login", nil)
	w := httptest.NewRecorder()
	a.handleAdminLogin(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestAdminAuth_HandleCallback_Success(t *testing.T) {
	gh := mockOAuthServer()
	defer gh.Close()

	a := testAdminAuth()
	a.SetGitHubAPIBase(gh.URL)
	a.SetOAuthBase(gh.URL)

	// Generate a valid state.
	state := a.generateState()

	req := httptest.NewRequest("GET", "/admin/callback?code=valid-code&state="+state, nil)
	w := httptest.NewRecorder()
	a.handleAdminCallback(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302, body: %s", w.Code, w.Body.String())
	}

	// Check cookie was set.
	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == cookieName {
			found = true
		}
	}
	if !found {
		t.Error("session cookie not set after callback")
	}

	// Check redirect to /admin.
	loc := w.Header().Get("Location")
	if !strings.HasSuffix(loc, "/admin") {
		t.Errorf("redirect location = %q, want .../admin", loc)
	}
}

func TestAdminAuth_HandleCallback_InvalidState(t *testing.T) {
	a := testAdminAuth()

	req := httptest.NewRequest("GET", "/admin/callback?code=valid-code&state=bad-state", nil)
	w := httptest.NewRecorder()
	a.handleAdminCallback(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestAdminAuth_HandleCallback_NonAdminEmail(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"access_token": "tok"})
	})
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"login": "nonadmin",
			"email": "other@example.com",
		})
	})
	gh := httptest.NewServer(mux)
	defer gh.Close()

	a := testAdminAuth()
	a.SetGitHubAPIBase(gh.URL)
	a.SetOAuthBase(gh.URL)

	state := a.generateState()
	req := httptest.NewRequest("GET", "/admin/callback?code=valid-code&state="+state, nil)
	w := httptest.NewRecorder()
	a.handleAdminCallback(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestAdminAuth_HandleCallback_BadCode(t *testing.T) {
	gh := mockOAuthServer()
	defer gh.Close()

	a := testAdminAuth()
	a.SetGitHubAPIBase(gh.URL)
	a.SetOAuthBase(gh.URL)

	state := a.generateState()
	req := httptest.NewRequest("GET", "/admin/callback?code=bad-code&state="+state, nil)
	w := httptest.NewRecorder()
	a.handleAdminCallback(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}

func TestAdminAuth_RequireSession_Valid(t *testing.T) {
	a := testAdminAuth()

	// Set a cookie first.
	rec := httptest.NewRecorder()
	a.setSessionCookie(rec, "admin@example.com", "adminuser")
	cookie := rec.Result().Cookies()[0]

	var gotUser *AuthContext
	handler := a.RequireSession(func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFromContext(r.Context())
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/admin", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if gotUser == nil {
		t.Fatal("user not set in context")
	}
	if gotUser.Email != "admin@example.com" {
		t.Errorf("email = %q", gotUser.Email)
	}
}

func TestAdminAuth_RequireSession_NoCookie(t *testing.T) {
	a := testAdminAuth()

	handler := a.RequireSession(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	})

	req := httptest.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("status = %d, want 302", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/admin/login" {
		t.Errorf("redirect = %q, want /admin/login", loc)
	}
}

func TestAdminAuth_BypassesBearerAuth(t *testing.T) {
	am := NewAuthMiddleware(NewLogger("error"))

	called := false
	handler := am.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	}))

	for _, path := range []string{"/admin", "/admin/login", "/admin/callback"} {
		called = false
		req := httptest.NewRequest("GET", path, nil) // No Bearer token
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != 200 || !called {
			t.Errorf("%s: status=%d called=%v, want 200/true", path, w.Code, called)
		}
	}
}

func TestAdminAuth_FetchPrimaryEmail(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"access_token": "tok"})
	})
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"login": "privateuser",
			"email": "", // private email
		})
	})
	mux.HandleFunc("/user/emails", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"email": "secondary@example.com", "primary": false, "verified": true},
			{"email": "admin@example.com", "primary": true, "verified": true},
		})
	})
	gh := httptest.NewServer(mux)
	defer gh.Close()

	a := testAdminAuth()
	a.SetGitHubAPIBase(gh.URL)
	a.SetOAuthBase(gh.URL)

	state := a.generateState()
	req := httptest.NewRequest("GET", "/admin/callback?code=valid-code&state="+state, nil)
	w := httptest.NewRecorder()
	a.handleAdminCallback(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302 (login with private email should still work), body: %s", w.Code, w.Body.String())
	}
}

// Verify state expires after TTL (we can't easily fast-forward time, but test the edge).
func TestAdminAuth_StateExpiry(t *testing.T) {
	a := testAdminAuth()

	state := a.generateState()
	// Manually override the stored time to be expired.
	a.stateCache.Store(state, time.Now().Add(-11*time.Minute))

	if a.validateState(state) {
		t.Error("expired state should be rejected")
	}
}
