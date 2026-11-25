package chat

import (
	"errors"
	"time"
)

var (
	ErrEmptyMessage    = errors.New("message content cannot be empty")
	ErrMessageLong     = errors.New("message exceeds character limit")
	ErrRateLimited     = errors.New("slow down! too many messages")
	ErrForbiddenAccess = errors.New("you are not a member of this workspace")
)

type ChatMessage struct {
	ID          string    `json:"id"`
	WorkspaceID *string   `json:"workspace_id,omitempty"`
	SenderID    string    `json:"sender_id"`
	SenderName  string    `json:"sender_name"`
	Picture     string    `json:"picture"`
	University  string    `json:"university,omitempty"`
	Content     string    `json:"content"`
	SentAt      time.Time `json:"sent_at"`
}

type SendMessageInput struct {
	Content string `json:"content"`
}