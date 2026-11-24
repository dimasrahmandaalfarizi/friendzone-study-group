package auth

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
)

const SessionCookieName = "app_session"

type Handler struct {
	service      *Service
	isProduction bool
}

func NewHandler(service *Service, isProduction bool) *Handler {
	return &Handler{
		service:      service,
		isProduction: isProduction,
	}
}

// Login godoc
// @Summary Start Google login
// @Description Creates OAuth state and PKCE verifier, then redirects to Google
// @Tags auth
// @Produce json
// @Success 307 {string} string "Redirects to Google"
// @Failure 500 {string} string "Failed to initiate Google login"
// @Router /auth/google/login [get]
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	authURL, state, verifier, err := h.service.BeginGoogleLogin()
	if err != nil {
		http.Error(w, "Failed to initiate Google login", http.StatusInternalServerError)
		return
	}

	h.setCookie(w, "oauth_state", state, "/auth/google", 300)
	h.setCookie(w, "oauth_verifier", verifier, "/auth/google", 300)

	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// Callback godoc
// @Summary Complete Google login
// @Description Validates OAuth state and ID token, creates a session, and redirects to the profile page
// @Tags auth
// @Param state query string true "OAuth state"
// @Param code query string true "Authorization code"
// @Success 303 {string} string "Redirects to /api/profile"
// @Failure 400 {string} string "State cookie not found"
// @Failure 401 {string} string "Invalid state or authentication failed"
// @Router /auth/google/callback [get]
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil {
		http.Error(w, "State cookie not found", http.StatusBadRequest)
		return
	}

	queryState := r.URL.Query().Get("state")
	if subtle.ConstantTimeCompare([]byte(stateCookie.Value), []byte(queryState)) != 1 {
		http.Error(w, "Invalid state parameter", http.StatusUnauthorized)
		return
	}

	verifierCookie, err := r.Cookie("oauth_verifier")
	if err != nil {
		http.Error(w, "Verifier cookie not found", http.StatusBadRequest)
		return
	}

	h.clearCookie(w, "oauth_state", "/auth/google")
	h.clearCookie(w, "oauth_verifier", "/auth/google")

	code := r.URL.Query().Get("code")
	sessionToken, err := h.service.CompleteGoogleLogin(r.Context(), code, verifierCookie.Value)
	if err != nil {
		http.Error(w, "Authentication failed", http.StatusUnauthorized)
		return
	}

	h.setCookie(w, SessionCookieName, sessionToken, "/", int(SessionTTL.Seconds()))
	http.Redirect(w, r, "/api/profile", http.StatusSeeOther)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(SessionCookieName); err == nil && cookie.Value != "" {
		_ = h.service.Logout(r.Context(), cookie.Value)
	}
	h.clearCookie(w, SessionCookieName, "/")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// LogoutAPI godoc
// @Summary Logout current session
// @Description Deletes the server-side Redis session and clears the app_session cookie
// @Tags auth
// @Produce json
// @Success 200 {object} map[string]bool "Logout result"
// @Router /auth/logout [post]
func (h *Handler) LogoutAPI(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(SessionCookieName); err == nil && cookie.Value != "" {
		_ = h.service.Logout(r.Context(), cookie.Value)
	}

	h.clearCookie(w, SessionCookieName, "/")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *Handler) setCookie(w http.ResponseWriter, name, value, path string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.isProduction,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearCookie(w http.ResponseWriter, name, path string) {
	h.setCookie(w, name, "", path, -1)
}
