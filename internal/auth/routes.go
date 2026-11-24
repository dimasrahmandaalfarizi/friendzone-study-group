package auth

import "net/http"

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /auth/google/login", h.Login)
	mux.HandleFunc("GET /auth/google/callback", h.Callback)
	mux.HandleFunc("GET /auth/logout", h.Logout)
	mux.HandleFunc("POST /auth/logout", h.LogoutAPI)
}
