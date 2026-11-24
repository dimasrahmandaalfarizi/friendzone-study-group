package auth

import (
	"context"
	"net/http"
)

type contextKey string

const UserIDContextKey contextKey = "authenticated_user_id"

func RequireAuth(authService *Service, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil || cookie.Value == "" {
			http.Error(w, "Unauthorized: please log in first", http.StatusUnauthorized)
			return
		}

		userID, err := authService.AuthenticateSession(r.Context(), cookie.Value)
		if err != nil {
			http.Error(w, "Unauthorized: invalid or expired session", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDContextKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDContextKey).(string)
	return userID, ok
}