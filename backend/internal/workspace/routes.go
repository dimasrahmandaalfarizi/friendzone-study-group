package workspace

import (
	"net/http"
	"github.com/BallSpins/friendzone/internal/auth"
)

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/workspaces/{workspaceID}", auth.RequireAuth(h.authService, h.GetWorkspace))
	mux.HandleFunc("GET /api/workspaces/{workspaceID}/streak", auth.RequireAuth(h.authService, h.GetStreak))

	// Collaborative Notes (Tiptap)
	mux.HandleFunc("GET /api/workspaces/{workspaceID}/notes", auth.RequireAuth(h.authService, h.ListNotes))
	mux.HandleFunc("POST /api/workspaces/{workspaceID}/notes", auth.RequireAuth(h.authService, h.CreateNote))
	mux.HandleFunc("PUT /api/workspaces/{workspaceID}/notes/{noteID}", auth.RequireAuth(h.authService, h.UpdateNote))

	// Session Scheduling
	mux.HandleFunc("GET /api/workspaces/{workspaceID}/sessions", auth.RequireAuth(h.authService, h.ListSessions))
	mux.HandleFunc("POST /api/workspaces/{workspaceID}/sessions", auth.RequireAuth(h.authService, h.CreateSession))
	mux.HandleFunc("PATCH /api/workspaces/{workspaceID}/sessions/{sessionID}/status", auth.RequireAuth(h.authService, h.UpdateSessionStatus))

	// Peer Accountability Evaluation
	mux.HandleFunc("POST /api/workspaces/{workspaceID}/sessions/{sessionID}/evaluate", auth.RequireAuth(h.authService, h.SubmitEvaluation))
}