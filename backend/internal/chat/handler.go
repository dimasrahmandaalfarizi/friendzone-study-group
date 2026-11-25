package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/BallSpins/friendzone/internal/auth"
	"github.com/redis/go-redis/v9"
)

type Handler struct {
	chatService *Service
	authService *auth.Service
}

func NewHandler(chatService *Service, authService *auth.Service) *Handler {
	return &Handler{
		chatService: chatService,
		authService: authService,
	}
}



// SendGlobal godoc
// @Summary Broadcast an ephemeral message to the global student chat stream
// @Tags chat
// @Accept json
// @Produce json
// @Param request body SendMessageInput true "Chat message payload"
// @Success 201 {object} ChatMessage
// @Router /api/chat/global [post]
func (h *Handler) SendGlobal(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserIDFromContext(r.Context())
	var in SendMessageInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	msg, err := h.chatService.PublishGlobal(r.Context(), userID, in.Content)
	if err != nil {
		switch {
		case errors.Is(err, ErrRateLimited):
			http.Error(w, err.Error(), http.StatusTooManyRequests)
		case errors.Is(err, ErrEmptyMessage), errors.Is(err, ErrMessageLong):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, "Failed to send message", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(msg)
}

// StreamGlobal godoc
// @Summary Subscribe to real-time Global Chat via SSE (ephemeral stream)
// @Tags chat
// @Produce text/event-stream
// @Success 200 {string} string "SSE event stream"
// @Router /api/chat/global/stream [get]
func (h *Handler) StreamGlobal(w http.ResponseWriter, r *http.Request) {
	pubsub := h.chatService.SubscribeGlobal(r.Context())
	defer pubsub.Close()
	serveSSE(w, r, pubsub)
}

// ListWorkspaceMessages godoc
// @Summary Get message history inside a shared workspace
// @Tags workspaces
// @Produce json
// @Param workspaceID path string true "Workspace UUID"
// @Success 200 {array} ChatMessage
// @Router /api/workspaces/{workspaceID}/messages [get]
func (h *Handler) ListWorkspaceMessages(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserIDFromContext(r.Context())
	msgs, err := h.chatService.ListWorkspaceMessages(r.Context(), r.PathValue("workspaceID"), userID, 50)
	if err != nil {
		if errors.Is(err, ErrForbiddenAccess) {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		http.Error(w, "Failed to load workspace messages", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(msgs)
}

// SendWorkspaceMessage godoc
// @Summary Send and persist a message inside a shared workspace
// @Tags workspaces
// @Accept json
// @Produce json
// @Param workspaceID path string true "Workspace UUID"
// @Param request body SendMessageInput true "Message payload"
// @Success 201 {object} ChatMessage
// @Router /api/workspaces/{workspaceID}/messages [post]
func (h *Handler) SendWorkspaceMessage(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserIDFromContext(r.Context())
	var in SendMessageInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	msg, err := h.chatService.SendWorkspaceMessage(r.Context(), r.PathValue("workspaceID"), userID, in.Content)
	if err != nil {
		switch {
		case errors.Is(err, ErrForbiddenAccess):
			http.Error(w, err.Error(), http.StatusForbidden)
		case errors.Is(err, ErrEmptyMessage), errors.Is(err, ErrMessageLong):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, "Failed to send message", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(msg)
}

// StreamWorkspace godoc
// @Summary Subscribe to real-time messages inside a shared workspace via SSE
// @Tags workspaces
// @Produce text/event-stream
// @Param workspaceID path string true "Workspace UUID"
// @Success 200 {string} string "SSE event stream"
// @Router /api/workspaces/{workspaceID}/messages/stream [get]
func (h *Handler) StreamWorkspace(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserIDFromContext(r.Context())
	pubsub, err := h.chatService.SubscribeWorkspace(r.Context(), r.PathValue("workspaceID"), userID)
	if err != nil {
		if errors.Is(err, ErrForbiddenAccess) {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		http.Error(w, "Failed to subscribe workspace chat", http.StatusInternalServerError)
		return
	}
	defer pubsub.Close()

	serveSSE(w, r, pubsub)
}

func serveSSE(w http.ResponseWriter, r *http.Request, pubsub *redis.PubSub) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	msgChan := pubsub.Channel()
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()

	_, _ = fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"ok\"}\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			_, _ = fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		case redisMsg, open := <-msgChan:
			if !open {
				return
			}
			_, _ = fmt.Fprintf(w, "event: message\ndata: %s\n\n", redisMsg.Payload)
			flusher.Flush()
		}
	}
}