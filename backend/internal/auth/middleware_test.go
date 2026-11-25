package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/BallSpins/friendzone/internal/config"
)

type mockSessionRepository struct {
	userID string
	err    error
	token  string
}

func (r *mockSessionRepository) Save(context.Context, string, string, time.Duration) error {
	return nil
}
func (r *mockSessionRepository) GetUserID(_ context.Context, token string) (string, error) {
	r.token = token
	return r.userID, r.err
}
func (r *mockSessionRepository) Delete(context.Context, string) error { return nil }

type mockUserProvisioner struct{}

func (mockUserProvisioner) SyncGoogleAccountID(context.Context, string, string, string, string) (string, error) {
	return "", nil
}

func testAuthService(sessions SessionRepository) *Service {
	return NewService(&config.Config{
		GoogleClientID:     "client",
		GoogleClientSecret: "secret",
		GoogleRedirectURL:  "http://localhost/callback",
	}, mockUserProvisioner{}, sessions)
}

func TestRequireAuthUnauthorizedCases(t *testing.T) {
	tests := []struct {
		name  string
		req   *http.Request
		store *mockSessionRepository
	}{
		{name: "missing cookie", req: httptest.NewRequest(http.MethodGet, "/", nil), store: &mockSessionRepository{}},
		{name: "empty cookie", req: httptest.NewRequest(http.MethodGet, "/", nil), store: &mockSessionRepository{}},
		{name: "session not found", req: httptest.NewRequest(http.MethodGet, "/", nil), store: &mockSessionRepository{err: ErrSessionNotFound}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "empty cookie" {
				tt.req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: ""})
			}
			if tt.name == "session not found" {
				tt.req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "missing"})
			}
			rec := httptest.NewRecorder()
			handler := RequireAuth(testAuthService(tt.store), func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("next handler must not run")
			})
			handler(rec, tt.req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", rec.Code)
			}
		})
	}
}

func TestRequireAuthPopulatesUserContext(t *testing.T) {
	store := &mockSessionRepository{userID: "u1"}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "valid-token"})
	rec := httptest.NewRecorder()
	called := false

	handler := RequireAuth(testAuthService(store), func(w http.ResponseWriter, r *http.Request) {
		called = true
		userID, ok := GetUserIDFromContext(r.Context())
		if !ok || userID != "u1" {
			t.Fatalf("unexpected auth context: %q, %v", userID, ok)
		}
		w.WriteHeader(http.StatusOK)
	})
	handler(rec, req)

	if !called || rec.Code != http.StatusOK {
		t.Fatalf("authenticated request failed: called=%v status=%d", called, rec.Code)
	}
	if store.token != "valid-token" {
		t.Fatalf("unexpected session token: %q", store.token)
	}
}
