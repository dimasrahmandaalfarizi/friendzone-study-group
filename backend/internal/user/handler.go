package user

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/BallSpins/friendzone/internal/auth"
)

type Handler struct {
	userService *Service
	authService *auth.Service
}

func NewHandler(userService *Service, authService *auth.Service) *Handler {
	return &Handler{
		userService: userService,
		authService: authService,
	}
}

// GetProfile godoc
// @Summary Get current user profile
// @Description Requires the app_session cookie
// @Tags users
// @Produce json
// @Success 200 {object} User
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "User not found"
// @Failure 500 {string} string "Internal server error"
// @Router /api/profile [get]
func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	profile, err := h.userService.GetProfile(r.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(profile)
}

// PatchProfile godoc
// @Summary Update basic profile fields
// @Description Updates the name and/or profile picture URL. Send an empty picture string to clear it.
// @Tags users
// @Accept json
// @Produce json
// @Param request body PatchProfileInput true "Fields to update"
// @Success 200 {object} User
// @Failure 400 {string} string "Invalid request payload"
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "User not found"
// @Failure 500 {string} string "Internal server error"
// @Router /api/profile [patch]
func (h *Handler) PatchProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var input PatchProfileInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	profile, err := h.userService.PatchBasicProfile(r.Context(), userID, input)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrInvalidProfilePatch) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(profile)
}

// UpsertAcademicProfile godoc
// @Summary Upsert academic profile and generate 768-d semantic embedding
// @Tags users
// @Accept json
// @Produce json
// @Param request body UpsertProfileInput true "Academic profile payload"
// @Success 200 {object} AcademicProfile
// @Failure 400 {string} string "Invalid request payload"
// @Failure 401 {string} string "Unauthorized"
// @Router /api/profile/academic [put]
func (h *Handler) UpsertAcademicProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var input UpsertProfileInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	profile, err := h.userService.UpdateAcademicProfile(r.Context(), userID, input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(profile)
}
