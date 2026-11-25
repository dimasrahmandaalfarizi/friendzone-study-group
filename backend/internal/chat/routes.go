package chat

import (
	"net/http"

	"github.com/BallSpins/friendzone/internal/auth"
)

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Global Chat (Ephemeral Real-Time Stream)
	mux.HandleFunc("GET /api/chat/global/stream", auth.RequireAuth(h.authService, h.StreamGlobal))
	mux.HandleFunc("POST /api/chat/global", auth.RequireAuth(h.authService, h.SendGlobal))

	// Private Workspace Chat (Persistent + Real-Time Stream)
	mux.HandleFunc("GET /api/workspaces/{workspaceID}/messages", auth.RequireAuth(h.authService, h.ListWorkspaceMessages))
	mux.HandleFunc("POST /api/workspaces/{workspaceID}/messages", auth.RequireAuth(h.authService, h.SendWorkspaceMessage))
	mux.HandleFunc("GET /api/workspaces/{workspaceID}/messages/stream", auth.RequireAuth(h.authService, h.StreamWorkspace))
}