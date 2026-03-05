package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUserFromContext_Anonymous(t *testing.T) {
	ctx := context.Background()
	user := UserFromContext(ctx)

	if user != AnonymousUser {
		t.Errorf("expected AnonymousUser, got %+v", user)
	}
	if user.ID != "anonymous" {
		t.Errorf("expected ID 'anonymous', got %q", user.ID)
	}
	if user.Provider != "none" {
		t.Errorf("expected Provider 'none', got %q", user.Provider)
	}
}

func TestContextWithUser_RoundTrip(t *testing.T) {
	original := &User{
		ID:       "user-123",
		Email:    "test@example.com",
		Name:     "Test User",
		Groups:   []string{"admins", "devs"},
		Provider: "oidc",
	}

	ctx := ContextWithUser(context.Background(), original)
	got := UserFromContext(ctx)

	if got.ID != original.ID {
		t.Errorf("expected ID %q, got %q", original.ID, got.ID)
	}
	if got.Email != original.Email {
		t.Errorf("expected Email %q, got %q", original.Email, got.Email)
	}
	if got.Provider != original.Provider {
		t.Errorf("expected Provider %q, got %q", original.Provider, got.Provider)
	}
	if len(got.Groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(got.Groups))
	}
}

func TestNoOpAuthenticator(t *testing.T) {
	auth := &NoOpAuthenticator{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	user, err := auth.Authenticate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user != AnonymousUser {
		t.Errorf("expected AnonymousUser, got %+v", user)
	}
}

func TestNoOpAuthorizer(t *testing.T) {
	authz := &NoOpAuthorizer{}
	ctx := context.Background()
	user := AnonymousUser

	if !authz.CanAccessRepo(ctx, user, "any/repo") {
		t.Error("NoOpAuthorizer should allow all repos")
	}

	repos := []string{"a", "b", "c"}
	filtered := authz.FilterRepos(ctx, user, repos)
	if len(filtered) != 3 {
		t.Errorf("expected all 3 repos, got %d", len(filtered))
	}
}

func TestAuthMiddleware_NoOpPassesThrough(t *testing.T) {
	auth := &NoOpAuthenticator{}
	var capturedUser *User

	handler := AuthMiddleware(auth)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if capturedUser == nil || capturedUser.ID != "anonymous" {
		t.Errorf("expected anonymous user in context, got %+v", capturedUser)
	}
}

type failingAuthenticator struct{}

func (f *failingAuthenticator) Authenticate(_ *http.Request) (*User, error) {
	return nil, http.ErrNoCookie // any non-nil error
}

func TestAuthMiddleware_RejectsOnError(t *testing.T) {
	auth := &failingAuthenticator{}

	handler := AuthMiddleware(auth)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when auth fails")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
