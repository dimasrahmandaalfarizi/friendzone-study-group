package match

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/BallSpins/friendzone/internal/auth"
)

type Handler struct {
	matchService *Service
	authService  *auth.Service
}

func NewHandler(matchService *Service, authService *auth.Service) *Handler {
	return &Handler{
		matchService: matchService,
		authService:  authService,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/discover", auth.RequireAuth(h.authService, h.Discover))
	mux.HandleFunc("POST /api/swipes", auth.RequireAuth(h.authService, h.Swipe))
	mux.HandleFunc("GET /api/matches", auth.RequireAuth(h.authService, h.ListMatches))
}

// Discover godoc
// @Summary Get Tinder-style discovery card deck ranked by semantic similarity
// @Description Returns up to 20 unswiped candidate profiles sorted by pgvector cosine similarity
// @Tags matching
// @Produce json
// @Param scope query string false "Discovery scope: all (cross-campus) or campus (intra-campus)" Enums(all, campus) default(all)
// @Success 200 {array} PeerCard
// @Failure 400 {string} string "Academic profile incomplete"
// @Failure 401 {string} string "Unauthorized"
// @Router /api/discover [get]
func (h *Handler) Discover(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	scope := r.URL.Query().Get("scope")
	cards, err := h.matchService.GetDiscoverDeck(r.Context(), userID, scope)
	if err != nil {
		if errors.Is(err, ErrProfileIncomplete) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "Failed to load discovery deck", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cards)
}

// Swipe godoc
// @Summary Swipe left (pass) or right (like) on a candidate card
// @Description If both users swipe right, creates a mutual match and shared workspace immediately
// @Tags matching
// @Accept json
// @Produce json
// @Param request body SwipeInput true "Swipe payload"
// @Success 200 {object} SwipeResult
// @Failure 400 {string} string "Invalid swipe payload"
// @Failure 401 {string} string "Unauthorized"
// @Router /api/swipes [post]
func (h *Handler) Swipe(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var input SwipeInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	res, err := h.matchService.Swipe(r.Context(), userID, input)
	if err != nil {
		if errors.Is(err, ErrInvalidDirection) || errors.Is(err, ErrSelfSwipe) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "Failed to record swipe", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// ListMatches godoc
// @Summary List mutual matches and their shared workspace IDs
// @Tags matching
// @Produce json
// @Success 200 {array} MutualMatch
// @Failure 401 {string} string "Unauthorized"
// @Router /api/matches [get]
func (h *Handler) ListMatches(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	matches, err := h.matchService.GetMutualMatches(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to load mutual matches", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(matches)
}