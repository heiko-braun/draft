package reviewd

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// AdminAuth handles GitHub OAuth login and session cookies for the admin UI.
type AdminAuth struct {
	clientID     string
	clientSecret string
	publicURL    string
	signingKey   [32]byte
	adminEmails  map[string]bool
	stateCache   sync.Map // map[string]time.Time
	client       *http.Client
	logger       *Logger

	// Configurable bases for testing.
	githubAPIBase string // default: https://api.github.com
	oauthBase     string // default: https://github.com
}

const (
	cookieName   = "_reviewd_admin"
	cookieMaxAge = 86400 // 24 hours
	stateTTL     = 10 * time.Minute
)

// NewAdminAuth creates an AdminAuth from the server config.
func NewAdminAuth(cfg Config, logger *Logger) *AdminAuth {
	emails := make(map[string]bool, len(cfg.AdminEmails))
	for _, e := range cfg.AdminEmails {
		emails[strings.ToLower(e)] = true
	}
	return &AdminAuth{
		clientID:      cfg.GitHubClientID,
		clientSecret:  cfg.GitHubClientSecret,
		publicURL:     cfg.PublicURL,
		signingKey:    sha256.Sum256([]byte("reviewd-session:" + cfg.GitHubClientSecret)),
		adminEmails:   emails,
		client:        &http.Client{Timeout: 10 * time.Second},
		logger:        logger,
		githubAPIBase: "https://api.github.com",
		oauthBase:     "https://github.com",
	}
}

// SetGitHubAPIBase overrides the GitHub API base URL (for testing).
func (a *AdminAuth) SetGitHubAPIBase(base string) { a.githubAPIBase = base }

// SetOAuthBase overrides the GitHub OAuth base URL (for testing).
func (a *AdminAuth) SetOAuthBase(base string) { a.oauthBase = base }

// --- Cookie signing ---

type sessionPayload struct {
	Email string `json:"email"`
	Login string `json:"login"`
	Exp   int64  `json:"exp"`
}

func (a *AdminAuth) setSessionCookie(w http.ResponseWriter, email, login string) {
	payload := sessionPayload{
		Email: email,
		Login: login,
		Exp:   time.Now().Add(time.Duration(cookieMaxAge) * time.Second).Unix(),
	}
	payloadJSON, _ := json.Marshal(payload)
	encoded := base64.RawURLEncoding.EncodeToString(payloadJSON)

	mac := hmac.New(sha256.New, a.signingKey[:])
	mac.Write([]byte(encoded))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	secure := !strings.HasPrefix(a.publicURL, "http://localhost")
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    encoded + "." + sig,
		Path:     "/admin",
		MaxAge:   cookieMaxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *AdminAuth) verifySessionCookie(r *http.Request) (email, login string, ok bool) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return "", "", false
	}

	idx := strings.LastIndex(cookie.Value, ".")
	if idx < 0 {
		return "", "", false
	}
	encoded := cookie.Value[:idx]
	sigStr := cookie.Value[idx+1:]

	// Verify HMAC.
	mac := hmac.New(sha256.New, a.signingKey[:])
	mac.Write([]byte(encoded))
	expectedSig := mac.Sum(nil)
	actualSig, err := base64.RawURLEncoding.DecodeString(sigStr)
	if err != nil || !hmac.Equal(expectedSig, actualSig) {
		return "", "", false
	}

	// Decode payload.
	payloadJSON, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", false
	}
	var p sessionPayload
	if err := json.Unmarshal(payloadJSON, &p); err != nil {
		return "", "", false
	}
	if time.Now().Unix() > p.Exp {
		return "", "", false
	}

	return p.Email, p.Login, true
}

// --- CSRF state ---

func (a *AdminAuth) generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	state := hex.EncodeToString(b)
	a.stateCache.Store(state, time.Now())
	return state
}

func (a *AdminAuth) validateState(state string) bool {
	v, ok := a.stateCache.LoadAndDelete(state)
	if !ok {
		return false
	}
	created := v.(time.Time)
	return time.Since(created) < stateTTL
}

// --- OAuth handlers ---

func (a *AdminAuth) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	if a.clientID == "" {
		http.Error(w, "GitHub OAuth not configured (GITHUB_CLIENT_ID not set)", http.StatusInternalServerError)
		return
	}

	state := a.generateState()
	redirectURI := a.publicURL + "/admin/callback"

	authURL := fmt.Sprintf("%s/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=user:email&state=%s",
		a.oauthBase,
		url.QueryEscape(a.clientID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(state),
	)

	http.Redirect(w, r, authURL, http.StatusFound)
}

func (a *AdminAuth) handleAdminCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || !a.validateState(state) {
		http.Error(w, "Invalid or expired OAuth state. Please try again.", http.StatusForbidden)
		return
	}

	// Exchange code for access token.
	token, err := a.exchangeCode(code)
	if err != nil {
		a.logger.Error("oauth token exchange failed", "error", err.Error())
		http.Error(w, "GitHub authentication failed. Please try again.", http.StatusBadGateway)
		return
	}

	// Get user info.
	login, email, err := a.fetchUser(token)
	if err != nil {
		a.logger.Error("oauth user fetch failed", "error", err.Error())
		http.Error(w, "Failed to retrieve GitHub user info.", http.StatusBadGateway)
		return
	}

	if !a.adminEmails[strings.ToLower(email)] {
		a.logger.Warn("admin access denied", "email", email, "login", login)
		http.Error(w, fmt.Sprintf("Access denied. %s is not an authorized admin.", email), http.StatusForbidden)
		return
	}

	a.logger.Info("admin login", "login", login, "email", email)
	a.setSessionCookie(w, email, login)
	http.Redirect(w, r, a.publicURL+"/admin", http.StatusFound)
}

func (a *AdminAuth) exchangeCode(code string) (string, error) {
	data := url.Values{
		"client_id":     {a.clientID},
		"client_secret": {a.clientSecret},
		"code":          {code},
		"redirect_uri":  {a.publicURL + "/admin/callback"},
	}

	req, _ := http.NewRequest("POST", a.oauthBase+"/login/oauth/access_token", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("github oauth: %s", result.Error)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("empty access token")
	}
	return result.AccessToken, nil
}

func (a *AdminAuth) fetchUser(token string) (login, email string, err error) {
	req, _ := http.NewRequest("GET", a.githubAPIBase+"/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("github api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("github returned %d", resp.StatusCode)
	}

	var user struct {
		Login string `json:"login"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", "", fmt.Errorf("decode user: %w", err)
	}

	email = user.Email
	if email == "" {
		email, err = a.fetchPrimaryEmail(token)
		if err != nil {
			return "", "", err
		}
	}

	return user.Login, email, nil
}

func (a *AdminAuth) fetchPrimaryEmail(token string) (string, error) {
	req, _ := http.NewRequest("GET", a.githubAPIBase+"/user/emails", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("github emails api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("github emails returned %d", resp.StatusCode)
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", fmt.Errorf("decode emails: %w", err)
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	return "", fmt.Errorf("no verified primary email found")
}

// --- Session middleware ---

// RequireSession checks for a valid admin session cookie.
// Redirects to /admin/login if missing or invalid.
func (a *AdminAuth) RequireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email, login, ok := a.verifySessionCookie(r)
		if !ok {
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}

		ctx := ContextWithAuth(r.Context(), &AuthContext{
			GitHubLogin: login,
			Email:       email,
		})
		next(w, r.WithContext(ctx))
	}
}
