package user

import (
	"errors"
	"time"
)

var (
	ErrUserNotFound        = errors.New("user not found")
	ErrInvalidProfilePatch = errors.New("invalid profile patch")
)

type User struct {
	ID        string    `json:"id"`
	GoogleSub string    `json:"google_sub"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Picture   string    `json:"picture"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AcademicProfile struct {
	UserID         string    `json:"user_id"`
	University     string    `json:"university"`
	Major          string    `json:"major"`
	TargetSubjects []string  `json:"target_subjects"`
	StudyMode      string    `json:"study_mode"` // "online", "in-person", "hybrid"
	Bio            string    `json:"bio"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type UserWithProfile struct {
	User
	Profile *AcademicProfile `json:"profile"`
}

type UpsertProfileInput struct {
	University     string   `json:"university"`
	Major          string   `json:"major"`
	TargetSubjects []string `json:"target_subjects"`
	StudyMode      string   `json:"study_mode"`
	Bio            string   `json:"bio"`
}

type PatchProfileInput struct {
	Name    *string `json:"name,omitempty"`
	Picture *string `json:"picture,omitempty"`
}
