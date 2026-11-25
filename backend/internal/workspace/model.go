package workspace

import (
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrWorkspaceNotFound = errors.New("workspace not found")
	ErrForbiddenAccess   = errors.New("you are not a member of this workspace")
	ErrNoteNotFound      = errors.New("note not found")
	ErrSessionNotFound   = errors.New("study session not found")
	ErrInvalidInput      = errors.New("invalid request input")
	ErrAlreadyEvaluated  = errors.New("you have already evaluated this session")
)

type WorkspaceDetail struct {
	ID          string    `json:"id"`
	MatchID     string    `json:"match_id"`
	Name        string    `json:"name"`
	PartnerID   string    `json:"partner_id"`
	PartnerName string    `json:"partner_name"`
	PartnerPic  string    `json:"partner_picture"`
	CreatedAt   time.Time `json:"created_at"`
}

type StreakStatus struct {
	WorkspaceID string `json:"workspace_id"`
	Current     int    `json:"current_streak"`
	ExpiresIn   int64  `json:"expires_in_seconds"`
}

type Note struct {
	ID          string          `json:"id"`
	WorkspaceID string          `json:"workspace_id"`
	Title       string          `json:"title"`
	ContentJSON json.RawMessage `json:"content_json" swaggertype:"object"`
	UpdatedBy   *string         `json:"updated_by,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type UpsertNoteInput struct {
	Title       string          `json:"title"`
	ContentJSON json.RawMessage `json:"content_json" swaggertype:"object"`
}

type StudySession struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	CreatedBy   string    `json:"created_by"`
	Topic       string    `json:"topic"`
	ScheduledAt time.Time `json:"scheduled_at"`
	MeetingLink string    `json:"meeting_link"`
	Status      string    `json:"status"` // "scheduled", "completed", "cancelled"
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateSessionInput struct {
	Topic       string    `json:"topic"`
	ScheduledAt time.Time `json:"scheduled_at"`
	MeetingLink string    `json:"meeting_link"`
}

type UpdateSessionStatusInput struct {
	Status string `json:"status"` // "completed" atau "cancelled"
}

type Evaluation struct {
	ID             string    `json:"id"`
	SessionID      string    `json:"session_id"`
	ReviewerID     string    `json:"reviewer_id"`
	RevieweeID     string    `json:"reviewee_id"`
	Rating         int       `json:"rating"`
	ReflectionNote string    `json:"reflection_note"`
	CreatedAt      time.Time `json:"created_at"`
}

type CreateEvaluationInput struct {
	Rating         int    `json:"rating"`
	ReflectionNote string `json:"reflection_note"`
}

