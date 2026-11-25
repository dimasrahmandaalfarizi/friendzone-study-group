package user

import (
	"net/http"

	"github.com/BallSpins/friendzone/internal/auth"
)

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/profile", auth.RequireAuth(h.authService, h.GetProfile))
	mux.HandleFunc("PATCH /api/profile", auth.RequireAuth(h.authService, h.PatchProfile))
	mux.HandleFunc("PUT /api/profile/academic", auth.RequireAuth(h.authService, h.UpsertAcademicProfile))
}