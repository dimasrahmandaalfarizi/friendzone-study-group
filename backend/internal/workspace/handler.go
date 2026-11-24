package workspace

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/BallSpins/friendzone/internal/auth"
)

type Handler struct {
	wsService   *Service
	authService *auth.Service
}

func NewHandler(wsService *Service, authService *auth.Service) *Handler {
	return &Handler{
		wsService:   wsService,
		authService: authService,
	}
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrWorkspaceNotFound), errors.Is(err, ErrNoteNotFound), errors.Is(err, ErrSessionNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, ErrForbiddenAccess):
		http.Error(w, err.Error(), http.StatusForbidden)
	case errors.Is(err, ErrAlreadyEvaluated):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, ErrInvalidInput):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// GetWorkspace godoc
// @Summary Get shared workspace details and study partner info
// @Tags workspaces
// @Produce json
// @Param workspaceID path string true "Workspace UUID"
// @Success 200 {object} WorkspaceDetail
// @Router /api/workspaces/{workspaceID} [get]
func (h *Handler) GetWorkspace(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserIDFromContext(r.Context())
	ws, err := h.wsService.GetWorkspace(r.Context(), r.PathValue("workspaceID"), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ws)
}

// ListNotes godoc
// @Summary List collaborative notes in a workspace
// @Tags workspaces
// @Produce json
// @Param workspaceID path string true "Workspace UUID"
// @Success 200 {array} Note
// @Router /api/workspaces/{workspaceID}/notes [get]
func (h *Handler) ListNotes(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserIDFromContext(r.Context())
	notes, err := h.wsService.ListNotes(r.Context(), r.PathValue("workspaceID"), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(notes)
}

// CreateNote godoc
// @Summary Create a new Tiptap collaborative note
// @Tags workspaces
// @Accept json
// @Produce json
// @Param workspaceID path string true "Workspace UUID"
// @Param request body UpsertNoteInput true "Note payload"
// @Success 201 {object} Note
// @Router /api/workspaces/{workspaceID}/notes [post]
func (h *Handler) CreateNote(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserIDFromContext(r.Context())
	var in UpsertNoteInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	note, err := h.wsService.CreateNote(r.Context(), r.PathValue("workspaceID"), userID, in)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(note)
}

// UpdateNote godoc
// @Summary Save updated Tiptap JSON content to an existing note
// @Tags workspaces
// @Accept json
// @Produce json
// @Param workspaceID path string true "Workspace UUID"
// @Param noteID path string true "Note UUID"
// @Param request body UpsertNoteInput true "Updated note payload"
// @Success 200 {object} Note
// @Router /api/workspaces/{workspaceID}/notes/{noteID} [put]
func (h *Handler) UpdateNote(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserIDFromContext(r.Context())
	var in UpsertNoteInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	note, err := h.wsService.UpdateNote(r.Context(), r.PathValue("workspaceID"), r.PathValue("noteID"), userID, in)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(note)
}

// ListSessions godoc
// @Summary List scheduled and completed study sessions
// @Tags workspaces
// @Produce json
// @Param workspaceID path string true "Workspace UUID"
// @Success 200 {array} StudySession
// @Router /api/workspaces/{workspaceID}/sessions [get]
func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserIDFromContext(r.Context())
	sessions, err := h.wsService.ListSessions(r.Context(), r.PathValue("workspaceID"), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sessions)
}

// CreateSession godoc
// @Summary Schedule a new study session
// @Tags workspaces
// @Accept json
// @Produce json
// @Param workspaceID path string true "Workspace UUID"
// @Param request body CreateSessionInput true "Session details"
// @Success 201 {object} StudySession
// @Router /api/workspaces/{workspaceID}/sessions [post]
func (h *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserIDFromContext(r.Context())
	var in CreateSessionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	session, err := h.wsService.CreateSession(r.Context(), r.PathValue("workspaceID"), userID, in)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(session)
}

// UpdateSessionStatus godoc
// @Summary Mark a study session as completed or cancelled
// @Tags workspaces
// @Accept json
// @Produce json
// @Param workspaceID path string true "Workspace UUID"
// @Param sessionID path string true "Session UUID"
// @Param request body UpdateSessionStatusInput true "New status"
// @Success 200 {object} StudySession
// @Router /api/workspaces/{workspaceID}/sessions/{sessionID}/status [patch]
func (h *Handler) UpdateSessionStatus(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserIDFromContext(r.Context())
	var in UpdateSessionStatusInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	session, err := h.wsService.UpdateSessionStatus(r.Context(), r.PathValue("workspaceID"), r.PathValue("sessionID"), userID, in.Status)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(session)
}

// SubmitEvaluation godoc
// @Summary Submit post-study peer evaluation rating and reflection note
// @Tags workspaces
// @Accept json
// @Produce json
// @Param workspaceID path string true "Workspace UUID"
// @Param sessionID path string true "Session UUID"
// @Param request body CreateEvaluationInput true "Peer evaluation payload"
// @Success 201 {object} Evaluation
// @Router /api/workspaces/{workspaceID}/sessions/{sessionID}/evaluate [post]
func (h *Handler) SubmitEvaluation(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserIDFromContext(r.Context())
	var in CreateEvaluationInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	ev, err := h.wsService.SubmitEvaluation(r.Context(), r.PathValue("workspaceID"), r.PathValue("sessionID"), userID, in)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(ev)
}