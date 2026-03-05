package middleware

import (
	"context"
	"net/http"
)

// contextKey is an unexported type for context keys in this package.
type contextKey int

const userContextKey contextKey = iota

// User represents an authenticated user.
type User struct {
	ID       string
	Email    string
	Name     string
	Groups   []string
	Provider string // "saml", "oidc", "local", "anonymous"
}

// AnonymousUser is the default user when no authentication is configured.
var AnonymousUser = &User{
	ID:       "anonymous",
	Name:     "Anonymous",
	Provider: "none",
}

// UserFromContext retrieves the authenticated user from the request context.
// Returns AnonymousUser if no user is set.
func UserFromContext(ctx context.Context) *User {
	if u, ok := ctx.Value(userContextKey).(*User); ok {
		return u
	}
	return AnonymousUser
}

// ContextWithUser returns a new context with the given user stored in it.
func ContextWithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// Authenticator validates incoming requests and returns user identity.
// The default NoOpAuthenticator always returns AnonymousUser.
// Enterprise implementations can provide SAML, OIDC, or other SSO providers.
type Authenticator interface {
	// Authenticate inspects the request (headers, cookies, tokens) and returns
	// the authenticated user. Return a non-nil error to reject the request
	// with 401 Unauthorized.
	Authenticate(r *http.Request) (*User, error)
}

// Authorizer controls access to resources based on user identity.
// The default NoOpAuthorizer allows all access.
// Enterprise implementations can provide RBAC or other access control.
type Authorizer interface {
	// CanAccessRepo checks if the user can view a repository.
	CanAccessRepo(ctx context.Context, user *User, repoName string) bool
	// FilterRepos filters a list of repos to only those the user can access.
	FilterRepos(ctx context.Context, user *User, repos []string) []string
}

// NoOpAuthenticator is the default authenticator that allows all requests
// and assigns AnonymousUser identity. Used in the open-source core.
type NoOpAuthenticator struct{}

// Authenticate always returns AnonymousUser with no error.
func (n *NoOpAuthenticator) Authenticate(_ *http.Request) (*User, error) {
	return AnonymousUser, nil
}

// NoOpAuthorizer is the default authorizer that allows all access.
// Used in the open-source core.
type NoOpAuthorizer struct{}

// CanAccessRepo always returns true.
func (n *NoOpAuthorizer) CanAccessRepo(_ context.Context, _ *User, _ string) bool {
	return true
}

// FilterRepos returns all repos unfiltered.
func (n *NoOpAuthorizer) FilterRepos(_ context.Context, _ *User, repos []string) []string {
	return repos
}

// AuthMiddleware returns HTTP middleware that authenticates each request
// using the provided Authenticator and stores the user in the request context.
func AuthMiddleware(auth Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := auth.Authenticate(r)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := ContextWithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
