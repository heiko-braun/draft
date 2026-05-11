package reviewd

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// AccessLevel represents the user's permission level for a repo.
type AccessLevel int

const (
	AccessNone  AccessLevel = iota
	AccessRead              // can view threads/comments
	AccessWrite             // can create/modify threads/comments
	AccessAdmin             // can delete, manage reviews
)

// RepoAccess holds cached permission information for a user+repo pair.
type RepoAccess struct {
	Level     AccessLevel
	ExpiresAt time.Time
}

// AuthMiddleware provides GitHub token verification and repo access checking.
type AuthMiddleware struct {
	// githubAPIBase is the GitHub API URL (default: https://api.github.com).
	githubAPIBase string

	// tokenCache maps token → GitHubUser, with expiry.
	tokenCache sync.Map // map[string]*tokenCacheEntry

	// repoAccessCache maps "token:owner/repo" → RepoAccess.
	repoAccessCache sync.Map // map[string]*RepoAccess

	// cacheTTL is how long cached entries live.
	cacheTTL time.Duration

	client *http.Client
	logger *Logger
}

type tokenCacheEntry struct {
	user      *AuthContext
	expiresAt time.Time
}

// NewAuthMiddleware creates an auth middleware with default settings.
func NewAuthMiddleware(logger *Logger) *AuthMiddleware {
	return &AuthMiddleware{
		githubAPIBase: "https://api.github.com",
		cacheTTL:      5 * time.Minute,
		client:        &http.Client{Timeout: 10 * time.Second},
		logger:        logger,
	}
}

// SetGitHubAPIBase overrides the GitHub API base URL (for testing).
func (am *AuthMiddleware) SetGitHubAPIBase(base string) {
	am.githubAPIBase = base
}

// Middleware returns an HTTP middleware that verifies the Bearer token.
func (am *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health endpoints.
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}

		token := extractBearerToken(r)
		if token == "" {
			am.logger.Debug("auth: no token", "path", r.URL.Path)
			writeErrorJSON(w, http.StatusUnauthorized, "missing or invalid Authorization header")
			return
		}

		user, err := am.verifyToken(token)
		if err != nil {
			am.logger.Warn("auth failed", "error", err.Error())
			writeErrorJSON(w, http.StatusUnauthorized, "invalid token")
			return
		}

		am.logger.Debug("auth: verified", "user", user.GitHubLogin, "participant_id", user.ParticipantID)
		ctx := ContextWithAuth(r.Context(), user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRepoAccess returns a middleware function that wraps a handler with a
// repo permission check. The returned function checks that the authenticated
// user has at least the given access level for the repo in the URL path.
func (am *AuthMiddleware) RequireRepoAccess(minLevel AccessLevel) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			owner := r.PathValue("owner")
			repo := r.PathValue("repo")
			if owner == "" || repo == "" {
				writeErrorJSON(w, http.StatusBadRequest, "missing owner or repo in path")
				return
			}

			token := extractBearerToken(r)
			level, err := am.checkRepoAccess(token, owner, repo)
			if err != nil {
				am.logger.Warn("repo access check failed", "error", err.Error())
				writeErrorJSON(w, http.StatusForbidden, "insufficient permissions")
				return
			}

			if level < minLevel {
				am.logger.Debug("access denied", "required", accessLevelString(minLevel), "actual", accessLevelString(level), "repo", owner+"/"+repo)
				writeErrorJSON(w, http.StatusForbidden, "insufficient permissions")
				return
			}

			am.logger.Debug("access granted", "level", accessLevelString(level), "repo", owner+"/"+repo)
			next(w, r)
		}
	}
}

func accessLevelString(l AccessLevel) string {
	switch l {
	case AccessRead:
		return "read"
	case AccessWrite:
		return "write"
	case AccessAdmin:
		return "admin"
	default:
		return "none"
	}
}

func (am *AuthMiddleware) verifyToken(token string) (*AuthContext, error) {
	// Use a hash as cache key to avoid storing raw tokens in memory.
	tokenKey := hashToken(token)

	// Check cache.
	if entry, ok := am.tokenCache.Load(tokenKey); ok {
		cached := entry.(*tokenCacheEntry)
		if time.Now().Before(cached.expiresAt) {
			am.logger.Debug("auth: token cache hit", "user", cached.user.GitHubLogin)
			return cached.user, nil
		}
		am.logger.Debug("auth: token cache expired")
		am.tokenCache.Delete(tokenKey)
	}

	am.logger.Debug("auth: calling GitHub /user API")
	// Call GitHub API.
	req, _ := http.NewRequest("GET", am.githubAPIBase+"/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := am.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("github returned %d", resp.StatusCode)
	}

	var ghUser struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
		return nil, fmt.Errorf("decode user: %w", err)
	}

	email := ghUser.Email
	if email == "" {
		email = ghUser.Login + "@users.noreply.github.com"
	}

	user := &AuthContext{
		GitHubLogin:   ghUser.Login,
		GitHubID:      ghUser.ID,
		Email:         email,
		Name:          ghUser.Name,
		ParticipantID: participantID(email),
	}

	am.tokenCache.Store(tokenKey, &tokenCacheEntry{
		user:      user,
		expiresAt: time.Now().Add(am.cacheTTL),
	})

	return user, nil
}

func (am *AuthMiddleware) checkRepoAccess(token, owner, repo string) (AccessLevel, error) {
	cacheKey := hashToken(token) + ":" + owner + "/" + repo

	if entry, ok := am.repoAccessCache.Load(cacheKey); ok {
		cached := entry.(*RepoAccess)
		if time.Now().Before(cached.ExpiresAt) {
			return cached.Level, nil
		}
		am.repoAccessCache.Delete(cacheKey)
	}

	req, _ := http.NewRequest("GET", am.githubAPIBase+"/repos/"+owner+"/"+repo, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := am.client.Do(req)
	if err != nil {
		return AccessNone, fmt.Errorf("github api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 || resp.StatusCode == 403 {
		return AccessNone, fmt.Errorf("no access to repo")
	}
	if resp.StatusCode != 200 {
		return AccessNone, fmt.Errorf("github returned %d", resp.StatusCode)
	}

	var repoResp struct {
		Permissions struct {
			Admin bool `json:"admin"`
			Push  bool `json:"push"`
			Pull  bool `json:"pull"`
		} `json:"permissions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&repoResp); err != nil {
		return AccessNone, fmt.Errorf("decode repo: %w", err)
	}

	level := AccessNone
	if repoResp.Permissions.Pull {
		level = AccessRead
	}
	if repoResp.Permissions.Push {
		level = AccessWrite
	}
	if repoResp.Permissions.Admin {
		level = AccessAdmin
	}

	am.repoAccessCache.Store(cacheKey, &RepoAccess{
		Level:     level,
		ExpiresAt: time.Now().Add(am.cacheTTL),
	})

	return level, nil
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(auth, "Bearer ")
}

func participantID(email string) string {
	h := sha256.Sum256([]byte(strings.ToLower(email)))
	return fmt.Sprintf("%x", h[:6])
}

// hashToken returns a SHA-256 hex digest of the token for use as a cache key,
// so raw tokens are not held in memory longer than necessary.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", h)
}

// AdminOnly returns a middleware that restricts access to configured admin emails.
func AdminOnly(adminEmails []string) func(http.HandlerFunc) http.HandlerFunc {
	allowed := make(map[string]bool, len(adminEmails))
	for _, e := range adminEmails {
		allowed[strings.ToLower(e)] = true
	}
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil || !allowed[strings.ToLower(user.Email)] {
				writeErrorJSON(w, http.StatusForbidden, "admin access required")
				return
			}
			next(w, r)
		}
	}
}

func writeErrorJSON(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
