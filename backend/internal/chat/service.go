package chat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const globalChannelKey = "chat:global:channel"

type Service struct {
	db  *sql.DB
	rdb *redis.Client
}

func NewService(db *sql.DB, rdb *redis.Client) *Service {
	return &Service{db: db, rdb: rdb}
}

func workspaceChannelKey(workspaceID string) string {
	return fmt.Sprintf("chat:ws:%s", workspaceID)
}

func (s *Service) VerifyWorkspaceMember(ctx context.Context, workspaceID, userID string) error {
	var isMember bool
	query := `
		SELECT EXISTS(
			SELECT 1
			FROM workspaces w
			JOIN matches m ON m.id = w.match_id
			WHERE w.id = $1 AND (m.user1_id = $2 OR m.user2_id = $2)
		)
	`
	if err := s.db.QueryRowContext(ctx, query, workspaceID, userID).Scan(&isMember); err != nil {
		return err
	}
	if !isMember {
		return ErrForbiddenAccess
	}
	return nil
}

// --- GLOBAL CHAT (EPHEMERAL STREAM, TANPA HISTORY) ---

func (s *Service) PublishGlobal(ctx context.Context, userID, rawContent string) (*ChatMessage, error) {
	content := strings.TrimSpace(rawContent)
	if content == "" {
		return nil, ErrEmptyMessage
	}
	if len([]rune(content)) > 500 {
		return nil, ErrMessageLong
	}

	rlKey := fmt.Sprintf("ratelimit:chat:global:%s", userID)
	if count, err := s.rdb.Incr(ctx, rlKey).Result(); err == nil {
		if count == 1 {
			_ = s.rdb.Expire(ctx, rlKey, 10*time.Second).Err()
		}
		if count > 5 {
			return nil, ErrRateLimited
		}
	}

	var name, picture, university string
	query := `
		SELECT u.name, u.picture, COALESCE(p.university, 'Independent Student')
		FROM users u
		LEFT JOIN user_profiles p ON p.user_id = u.id
		WHERE u.id = $1
	`
	if err := s.db.QueryRowContext(ctx, query, userID).Scan(&name, &picture, &university); err != nil {
		return nil, fmt.Errorf("fetch sender profile: %w", err)
	}

	msg := ChatMessage{
		ID:         uuid.NewString(),
		SenderID:   userID,
		SenderName: name,
		Picture:    picture,
		University: university,
		Content:    content,
		SentAt:     time.Now().UTC(),
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	if err := s.rdb.Publish(ctx, globalChannelKey, payload).Err(); err != nil {
		return nil, fmt.Errorf("publish global message: %w", err)
	}

	return &msg, nil
}

func (s *Service) SubscribeGlobal(ctx context.Context) *redis.PubSub {
	return s.rdb.Subscribe(ctx, globalChannelKey)
}

// --- WORKSPACE CHAT (PERSISTENT POSTGRES + REAL-TIME REDIS PUB/SUB) ---

func (s *Service) SendWorkspaceMessage(ctx context.Context, workspaceID, userID, rawContent string) (*ChatMessage, error) {
	if err := s.VerifyWorkspaceMember(ctx, workspaceID, userID); err != nil {
		return nil, err
	}

	content := strings.TrimSpace(rawContent)
	if content == "" {
		return nil, ErrEmptyMessage
	}
	if len([]rune(content)) > 1000 {
		return nil, ErrMessageLong
	}

	var msg ChatMessage
	msg.WorkspaceID = &workspaceID
	msg.SenderID = userID
	msg.Content = content

	query := `
		WITH inserted AS (
			INSERT INTO workspace_messages (workspace_id, sender_id, content)
			VALUES ($1, $2, $3)
			RETURNING id, created_at
		)
		SELECT i.id, i.created_at, u.name, u.picture
		FROM inserted i
		JOIN users u ON u.id = $2
	`
	if err := s.db.QueryRowContext(ctx, query, workspaceID, userID, content).Scan(
		&msg.ID, &msg.SentAt, &msg.SenderName, &msg.Picture,
	); err != nil {
		return nil, fmt.Errorf("insert workspace message: %w", err)
	}

	if payload, err := json.Marshal(msg); err == nil {
		_ = s.rdb.Publish(ctx, workspaceChannelKey(workspaceID), payload).Err()
	}

	return &msg, nil
}

func (s *Service) ListWorkspaceMessages(ctx context.Context, workspaceID, userID string, limit int) ([]ChatMessage, error) {
	if err := s.VerifyWorkspaceMember(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := `
		SELECT id, workspace_id, sender_id, sender_name, picture, content, created_at
		FROM (
			SELECT
				wm.id,
				wm.workspace_id,
				wm.sender_id,
				u.name AS sender_name,
				u.picture,
				wm.content,
				wm.created_at
			FROM workspace_messages wm
			JOIN users u ON u.id = wm.sender_id
			WHERE wm.workspace_id = $1
			ORDER BY wm.created_at DESC
			LIMIT $2
		) sub
		ORDER BY created_at ASC
	`
	rows, err := s.db.QueryContext(ctx, query, workspaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]ChatMessage, 0)
	for rows.Next() {
		var m ChatMessage
		var wsID string
		if err := rows.Scan(&m.ID, &wsID, &m.SenderID, &m.SenderName, &m.Picture, &m.Content, &m.SentAt); err != nil {
			return nil, err
		}
		m.WorkspaceID = &wsID
		messages = append(messages, m)
	}

	return messages, rows.Err()
}

func (s *Service) SubscribeWorkspace(ctx context.Context, workspaceID, userID string) (*redis.PubSub, error) {
	if err := s.VerifyWorkspaceMember(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	return s.rdb.Subscribe(ctx, workspaceChannelKey(workspaceID)), nil
}