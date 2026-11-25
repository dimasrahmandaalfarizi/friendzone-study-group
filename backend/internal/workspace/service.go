package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Repository interface {
	GetWorkspaceAndPartner(ctx context.Context, workspaceID, userID string) (*WorkspaceDetail, error)
	ListNotes(ctx context.Context, workspaceID string) ([]Note, error)
	CreateNote(ctx context.Context, workspaceID, userID string, in UpsertNoteInput) (*Note, error)
	UpdateNote(ctx context.Context, workspaceID, noteID, userID string, in UpsertNoteInput) (*Note, error)
	ListSessions(ctx context.Context, workspaceID string) ([]StudySession, error)
	CreateSession(ctx context.Context, workspaceID, userID string, in CreateSessionInput) (*StudySession, error)
	UpdateSessionStatus(ctx context.Context, workspaceID, sessionID, status string) (*StudySession, error)
	CreateEvaluation(ctx context.Context, workspaceID, sessionID, reviewerID, revieweeID string, in CreateEvaluationInput) (*Evaluation, error)
	CountSessionEvaluations(ctx context.Context, sessionID string) (int, error)
}

type Service struct {
	repo Repository
	rdb  *redis.Client
}

func NewService(repo Repository, rdb *redis.Client) *Service {
	return &Service{repo: repo, rdb: rdb}
}

func streakRedisKey(workspaceID string) string {
	return fmt.Sprintf("streak:ws:%s", workspaceID)
}

func (s *Service) GetWorkspace(ctx context.Context, workspaceID, userID string) (*WorkspaceDetail, error) {
	return s.repo.GetWorkspaceAndPartner(ctx, workspaceID, userID)
}

func (s *Service) GetWorkspaceStreak(ctx context.Context, workspaceID, userID string) (*StreakStatus, error) {
	if _, err := s.repo.GetWorkspaceAndPartner(ctx, workspaceID, userID); err != nil {
		return nil, err
	}

	key := streakRedisKey(workspaceID)
	count, err := s.rdb.HGet(ctx, key, "count").Int()
	if errors.Is(err, redis.Nil) {
		return &StreakStatus{WorkspaceID: workspaceID, Current: 0, ExpiresIn: 0}, nil
	}
	if err != nil {
		return nil, err
	}

	ttl, _ := s.rdb.TTL(ctx, key).Result()
	expSec := int64(ttl.Seconds())
	if expSec < 0 {
		expSec = 0
	}

	return &StreakStatus{
		WorkspaceID: workspaceID,
		Current:     count,
		ExpiresIn:   expSec,
	}, nil
}

func (s *Service) ListNotes(ctx context.Context, workspaceID, userID string) ([]Note, error) {
	if _, err := s.repo.GetWorkspaceAndPartner(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	return s.repo.ListNotes(ctx, workspaceID)
}

func (s *Service) CreateNote(ctx context.Context, workspaceID, userID string, in UpsertNoteInput) (*Note, error) {
	if _, err := s.repo.GetWorkspaceAndPartner(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		in.Title = "Untitled Study Note"
	}
	if len(in.ContentJSON) == 0 {
		in.ContentJSON = json.RawMessage(`{"type":"doc","content":[]}`)
	} else if !json.Valid(in.ContentJSON) {
		return nil, fmt.Errorf("%w: content_json must be valid JSON", ErrInvalidInput)
	}

	return s.repo.CreateNote(ctx, workspaceID, userID, in)
}

func (s *Service) UpdateNote(ctx context.Context, workspaceID, noteID, userID string, in UpsertNoteInput) (*Note, error) {
	if _, err := s.repo.GetWorkspaceAndPartner(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return nil, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}
	if len(in.ContentJSON) == 0 || !json.Valid(in.ContentJSON) {
		return nil, fmt.Errorf("%w: content_json must be valid JSON", ErrInvalidInput)
	}

	return s.repo.UpdateNote(ctx, workspaceID, noteID, userID, in)
}

func (s *Service) ListSessions(ctx context.Context, workspaceID, userID string) ([]StudySession, error) {
	if _, err := s.repo.GetWorkspaceAndPartner(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	return s.repo.ListSessions(ctx, workspaceID)
}

func (s *Service) CreateSession(ctx context.Context, workspaceID, userID string, in CreateSessionInput) (*StudySession, error) {
	if _, err := s.repo.GetWorkspaceAndPartner(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	in.Topic = strings.TrimSpace(in.Topic)
	in.MeetingLink = strings.TrimSpace(in.MeetingLink)

	if in.Topic == "" || in.ScheduledAt.IsZero() {
		return nil, fmt.Errorf("%w: topic and scheduled_at are required", ErrInvalidInput)
	}

	return s.repo.CreateSession(ctx, workspaceID, userID, in)
}

func (s *Service) UpdateSessionStatus(ctx context.Context, workspaceID, sessionID, userID, rawStatus string) (*StudySession, error) {
	if _, err := s.repo.GetWorkspaceAndPartner(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	status := strings.ToLower(strings.TrimSpace(rawStatus))
	if !slices.Contains([]string{"completed", "cancelled"}, status) {
		return nil, fmt.Errorf("%w: status must be completed or cancelled", ErrInvalidInput)
	}

	return s.repo.UpdateSessionStatus(ctx, workspaceID, sessionID, status)
}

func (s *Service) SubmitEvaluation(ctx context.Context, workspaceID, sessionID, reviewerID string, in CreateEvaluationInput) (*Evaluation, error) {
	ws, err := s.repo.GetWorkspaceAndPartner(ctx, workspaceID, reviewerID)
	if err != nil {
		return nil, err
	}
	in.ReflectionNote = strings.TrimSpace(in.ReflectionNote)

	if in.Rating < 1 || in.Rating > 5 {
		return nil, fmt.Errorf("%w: rating must be between 1 and 5", ErrInvalidInput)
	}
	if in.ReflectionNote == "" {
		return nil, fmt.Errorf("%w: reflection_note is required for accountability", ErrInvalidInput)
	}

	ev, err := s.repo.CreateEvaluation(ctx, workspaceID, sessionID, reviewerID, ws.PartnerID, in)
	if err != nil {
		return nil, err
	}

	// Streak bertambah hanya jika kedua peer (count == 2) telah mengisi evaluasi pasca-sesi
	evalCount, err := s.repo.CountSessionEvaluations(ctx, sessionID)
	if err == nil && evalCount >= 2 {
		key := streakRedisKey(workspaceID)
		lastSession, _ := s.rdb.HGet(ctx, key, "last_session_id").Result()
		if lastSession != sessionID {
			pipe := s.rdb.TxPipeline()
			pipe.HIncrBy(ctx, key, "count", 1)
			pipe.HSet(ctx, key, "last_session_id", sessionID, "updated_at", time.Now().UTC().Format(time.RFC3339))
			pipe.Expire(ctx, key, 72*time.Hour)
			_, _ = pipe.Exec(ctx)
		}
	}

	return ev, nil
}