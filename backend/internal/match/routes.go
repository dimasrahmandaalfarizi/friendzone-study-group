package match

import (
	"net/http"

	"github.com/BallSpins/friendzone/internal/auth"
)

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/discover", auth.RequireAuth(h.authService, h.Discover))
	mux.HandleFunc("POST /api/swipes", auth.RequireAuth(h.authService, h.Swipe))
	mux.HandleFunc("GET /api/matches", auth.RequireAuth(h.authService, h.ListMatches))
}