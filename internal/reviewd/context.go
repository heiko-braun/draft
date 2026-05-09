package reviewd

import "context"

// AuthContext holds the authenticated user's identity and permissions.
type AuthContext struct {
	// GitHubLogin is the user's GitHub username.
	GitHubLogin string

	// GitHubID is the user's numeric GitHub ID.
	GitHubID int64

	// Email is the user's primary email.
	Email string

	// Name is the user's display name.
	Name string

	// ParticipantID is the hash-based ID matching the review participant scheme.
	ParticipantID string
}

type contextKey struct{}

// ContextWithAuth returns a new context with the auth context attached.
func ContextWithAuth(ctx context.Context, auth *AuthContext) context.Context {
	return context.WithValue(ctx, contextKey{}, auth)
}

// UserFromContext retrieves the AuthContext from the request context.
// Returns nil if not authenticated.
func UserFromContext(ctx context.Context) *AuthContext {
	v, _ := ctx.Value(contextKey{}).(*AuthContext)
	return v
}
