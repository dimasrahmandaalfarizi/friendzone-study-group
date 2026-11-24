package match

import (
	"errors"
	"time"
)

var (
	ErrProfileIncomplete = errors.New("complete your academic profile before discovering peers")
	ErrInvalidDirection  = errors.New("direction must be 'left' or 'right'")
	ErrSelfSwipe         = errors.New("cannot swipe on your own profile")
)

type PeerCard struct {
	UserID           string   `json:"user_id"`
	Name             string   `json:"name"`
	Picture          string   `json:"picture"`
	University       string   `json:"university"`
	Major            string   `json:"major"`
	TargetSubjects   []string `json:"target_subjects"`
	StudyMode        string   `json:"study_mode"`
	Bio              string   `json:"bio"`
	CompatibilityPct float64  `json:"compatibility_pct"`
}

type SwipeInput struct {
	TargetUserID string `json:"target_user_id"`
	Direction    string `json:"direction"` // "right" atau "left"
}

type SwipeResult struct {
	Matched     bool    `json:"matched"`
	MatchID     *string `json:"match_id,omitempty"`
	WorkspaceID *string `json:"workspace_id,omitempty"`
}

type MutualMatch struct {
	MatchID     string    `json:"match_id"`
	WorkspaceID string    `json:"workspace_id"`
	PartnerID   string    `json:"partner_id"`
	PartnerName string    `json:"partner_name"`
	Picture     string    `json:"picture"`
	University  string    `json:"university"`
	Major       string    `json:"major"`
	MatchedAt   time.Time `json:"matched_at"`
}