package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type mockWorkspaceRepository struct {
	workspace          *WorkspaceDetail
	workspaceErr       error
	note               *Note
	session            *StudySession
	evaluation         *Evaluation
	count              int
	createNoteInput    UpsertNoteInput
	createSessionInput CreateSessionInput
	revieweeID         string
}

func (r *mockWorkspaceRepository) GetWorkspaceAndPartner(context.Context, string, string) (*WorkspaceDetail, error) {
	return r.workspace, r.workspaceErr
}
func (r *mockWorkspaceRepository) ListNotes(context.Context, string) ([]Note, error) { return nil, nil }
func (r *mockWorkspaceRepository) CreateNote(_ context.Context, _, _ string, in UpsertNoteInput) (*Note, error) {
	r.createNoteInput = in
	return r.note, nil
}
func (r *mockWorkspaceRepository) UpdateNote(context.Context, string, string, string, UpsertNoteInput) (*Note, error) {
	return &Note{}, nil
}
func (r *mockWorkspaceRepository) ListSessions(context.Context, string) ([]StudySession, error) {
	return nil, nil
}
func (r *mockWorkspaceRepository) CreateSession(_ context.Context, _, _ string, in CreateSessionInput) (*StudySession, error) {
	r.createSessionInput = in
	return r.session, nil
}
func (r *mockWorkspaceRepository) UpdateSessionStatus(context.Context, string, string, string) (*StudySession, error) {
	return &StudySession{}, nil
}
func (r *mockWorkspaceRepository) CreateEvaluation(_ context.Context, _, _, _, revieweeID string, _ CreateEvaluationInput) (*Evaluation, error) {
	r.revieweeID = revieweeID
	return r.evaluation, nil
}
func (r *mockWorkspaceRepository) CountSessionEvaluations(context.Context, string) (int, error) {
	return r.count, nil
}

func newWorkspaceRedis(t *testing.T) *redis.Client {
	t.Helper()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestWorkspaceAuthorization(t *testing.T) {
	rdb := newWorkspaceRedis(t)
	repo := &mockWorkspaceRepository{workspaceErr: ErrForbiddenAccess}
	service := NewService(repo, rdb)
	ctx := context.Background()

	if _, err := service.GetWorkspace(ctx, "ws1", "outsider"); !errors.Is(err, ErrForbiddenAccess) {
		t.Fatalf("GetWorkspace: %v", err)
	}
	if _, err := service.GetWorkspaceStreak(ctx, "ws1", "outsider"); !errors.Is(err, ErrForbiddenAccess) {
		t.Fatalf("GetWorkspaceStreak: %v", err)
	}
	if _, err := service.ListNotes(ctx, "ws1", "outsider"); !errors.Is(err, ErrForbiddenAccess) {
		t.Fatalf("ListNotes: %v", err)
	}
	if _, err := service.CreateNote(ctx, "ws1", "outsider", UpsertNoteInput{}); !errors.Is(err, ErrForbiddenAccess) {
		t.Fatalf("CreateNote: %v", err)
	}
	if _, err := service.UpdateNote(ctx, "ws1", "note1", "outsider", UpsertNoteInput{}); !errors.Is(err, ErrForbiddenAccess) {
		t.Fatalf("UpdateNote: %v", err)
	}
	if _, err := service.ListSessions(ctx, "ws1", "outsider"); !errors.Is(err, ErrForbiddenAccess) {
		t.Fatalf("ListSessions: %v", err)
	}
	if _, err := service.CreateSession(ctx, "ws1", "outsider", CreateSessionInput{}); !errors.Is(err, ErrForbiddenAccess) {
		t.Fatalf("CreateSession: %v", err)
	}
	if _, err := service.UpdateSessionStatus(ctx, "ws1", "session1", "outsider", "completed"); !errors.Is(err, ErrForbiddenAccess) {
		t.Fatalf("UpdateSessionStatus: %v", err)
	}
	if _, err := service.SubmitEvaluation(ctx, "ws1", "session1", "outsider", CreateEvaluationInput{Rating: 5, ReflectionNote: "Good"}); !errors.Is(err, ErrForbiddenAccess) {
		t.Fatalf("SubmitEvaluation: %v", err)
	}
}

func TestCreateNoteDefaultsAndJSONValidation(t *testing.T) {
	rdb := newWorkspaceRedis(t)
	repo := &mockWorkspaceRepository{
		workspace: &WorkspaceDetail{ID: "ws1", PartnerID: "u2"},
		note:      &Note{},
	}
	service := NewService(repo, rdb)

	if _, err := service.CreateNote(context.Background(), "ws1", "u1", UpsertNoteInput{}); err != nil {
		t.Fatalf("CreateNote returned error: %v", err)
	}
	if repo.createNoteInput.Title != "Untitled Study Note" {
		t.Fatalf("unexpected title: %q", repo.createNoteInput.Title)
	}
	if string(repo.createNoteInput.ContentJSON) != `{"type":"doc","content":[]}` {
		t.Fatalf("unexpected default content: %s", repo.createNoteInput.ContentJSON)
	}
	if _, err := service.CreateNote(context.Background(), "ws1", "u1", UpsertNoteInput{Title: "Bad", ContentJSON: json.RawMessage(`{"broken"`)}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for malformed JSON, got %v", err)
	}
}

func TestUpdateNoteRequiresTitleAndValidJSON(t *testing.T) {
	rdb := newWorkspaceRedis(t)
	repo := &mockWorkspaceRepository{workspace: &WorkspaceDetail{ID: "ws1", PartnerID: "u2"}}
	service := NewService(repo, rdb)

	if _, err := service.UpdateNote(context.Background(), "ws1", "note1", "u1", UpsertNoteInput{Title: " ", ContentJSON: json.RawMessage(`{}`)}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected title error, got %v", err)
	}
	if _, err := service.UpdateNote(context.Background(), "ws1", "note1", "u1", UpsertNoteInput{Title: "N", ContentJSON: json.RawMessage(`x`)}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected JSON error, got %v", err)
	}
}

func TestCreateAndUpdateSessionValidation(t *testing.T) {
	rdb := newWorkspaceRedis(t)
	repo := &mockWorkspaceRepository{
		workspace: &WorkspaceDetail{ID: "ws1", PartnerID: "u2"},
		session:   &StudySession{},
	}
	service := NewService(repo, rdb)

	if _, err := service.CreateSession(context.Background(), "ws1", "u1", CreateSessionInput{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected required-field error, got %v", err)
	}
	if _, err := service.CreateSession(context.Background(), "ws1", "u1", CreateSessionInput{Topic: "Math", ScheduledAt: time.Now(), MeetingLink: ""}); err != nil {
		t.Fatalf("in-person session should allow empty meeting link: %v", err)
	}
	if _, err := service.UpdateSessionStatus(context.Background(), "ws1", "s1", "u1", "scheduled"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected status error, got %v", err)
	}
	if _, err := service.UpdateSessionStatus(context.Background(), "ws1", "s1", "u1", " COMPLETED "); err != nil {
		t.Fatalf("completed status should be accepted: %v", err)
	}
}

func TestSubmitEvaluationValidationAndPartnerReviewee(t *testing.T) {
	rdb := newWorkspaceRedis(t)
	repo := &mockWorkspaceRepository{
		workspace:  &WorkspaceDetail{ID: "ws1", PartnerID: "u2"},
		evaluation: &Evaluation{},
	}
	service := NewService(repo, rdb)

	if _, err := service.SubmitEvaluation(context.Background(), "ws1", "s1", "u1", CreateEvaluationInput{Rating: 0, ReflectionNote: "ok"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected rating error, got %v", err)
	}
	if _, err := service.SubmitEvaluation(context.Background(), "ws1", "s1", "u1", CreateEvaluationInput{Rating: 6, ReflectionNote: "ok"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected rating error, got %v", err)
	}
	if _, err := service.SubmitEvaluation(context.Background(), "ws1", "s1", "u1", CreateEvaluationInput{Rating: 5, ReflectionNote: " "}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected reflection error, got %v", err)
	}
	if _, err := service.SubmitEvaluation(context.Background(), "ws1", "s1", "u1", CreateEvaluationInput{Rating: 5, ReflectionNote: "  clear  "}); err != nil {
		t.Fatalf("valid evaluation returned error: %v", err)
	}
	if repo.revieweeID != "u2" {
		t.Fatalf("reviewee must be partner u2, got %q", repo.revieweeID)
	}
}

func TestStreakEngineFirstSecondAndIdempotent(t *testing.T) {
	rdb := newWorkspaceRedis(t)
	ctx := context.Background()
	repo := &mockWorkspaceRepository{
		workspace:  &WorkspaceDetail{ID: "ws1", PartnerID: "u2"},
		evaluation: &Evaluation{},
		count:      1,
	}
	service := NewService(repo, rdb)

	if _, err := service.SubmitEvaluation(ctx, "ws1", "s1", "u1", CreateEvaluationInput{Rating: 5, ReflectionNote: "ok"}); err != nil {
		t.Fatal(err)
	}
	if rdb.Exists(ctx, streakRedisKey("ws1")).Val() > 0 {
		t.Fatal("first evaluation must not create streak")
	}

	repo.count = 2
	if _, err := service.SubmitEvaluation(ctx, "ws1", "s1", "u2", CreateEvaluationInput{Rating: 5, ReflectionNote: "ok"}); err != nil {
		t.Fatal(err)
	}
	count, _ := rdb.HGet(ctx, streakRedisKey("ws1"), "count").Int()
	if count != 1 {
		t.Fatalf("expected streak count 1, got %d", count)
	}
	last, _ := rdb.HGet(ctx, streakRedisKey("ws1"), "last_session_id").Result()
	if last != "s1" {
		t.Fatalf("unexpected last session: %q", last)
	}
	ttl, _ := rdb.TTL(ctx, streakRedisKey("ws1")).Result()
	if ttl <= 71*time.Hour || ttl > 72*time.Hour {
		t.Fatalf("unexpected streak TTL: %v", ttl)
	}

	if _, err := service.SubmitEvaluation(ctx, "ws1", "s1", "u1", CreateEvaluationInput{Rating: 5, ReflectionNote: "again"}); err != nil {
		t.Fatal(err)
	}
	count, _ = rdb.HGet(ctx, streakRedisKey("ws1"), "count").Int()
	if count != 1 {
		t.Fatalf("duplicate evaluation increased streak twice: %d", count)
	}
}

func TestGetWorkspaceStreakMissingAndExpired(t *testing.T) {
	rdb := newWorkspaceRedis(t)
	repo := &mockWorkspaceRepository{workspace: &WorkspaceDetail{ID: "ws1", PartnerID: "u2"}}
	service := NewService(repo, rdb)

	status, err := service.GetWorkspaceStreak(context.Background(), "ws1", "u1")
	if err != nil || status.Current != 0 || status.ExpiresIn != 0 {
		t.Fatalf("unexpected missing streak status: %+v, %v", status, err)
	}
}
