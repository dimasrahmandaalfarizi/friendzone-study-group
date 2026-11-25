package match

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/pgvector/pgvector-go"
	"github.com/redis/go-redis/v9"
)

type mockMatchRepository struct {
	vec          pgvector.Vector
	campus       string
	embeddingErr error
	cards        []PeerCard
	fetchErr     error
	swipeResult  *SwipeResult
	swipeErr     error
	filter       string
	limit        int
}

func (r *mockMatchRepository) GetUserEmbeddingAndCampus(context.Context, string) (pgvector.Vector, string, error) {
	return r.vec, r.campus, r.embeddingErr
}
func (r *mockMatchRepository) FetchDiscoveryDeck(_ context.Context, _ string, _ pgvector.Vector, universityFilter string, limit int) ([]PeerCard, error) {
	r.filter = universityFilter
	r.limit = limit
	return r.cards, r.fetchErr
}
func (r *mockMatchRepository) RecordSwipe(context.Context, string, string, string) (*SwipeResult, error) {
	return r.swipeResult, r.swipeErr
}
func (r *mockMatchRepository) ListMutualMatches(context.Context, string) ([]MutualMatch, error) {
	return nil, nil
}

func newMatchRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return srv, client
}

func TestGetDiscoverDeckCacheHit(t *testing.T) {
	_, rdb := newMatchRedis(t)
	repo := &mockMatchRepository{}
	service := NewService(repo, rdb)
	cards := []PeerCard{{UserID: "peer-1", Name: "Peer"}}
	raw, _ := json.Marshal(cards)

	_ = rdb.Set(context.Background(), deckCacheKey("u1", "all"), raw, time.Minute).Err()
	got, err := service.GetDiscoverDeck(context.Background(), "u1", "ALL")
	if err != nil {
		t.Fatalf("GetDiscoverDeck returned error: %v", err)
	}
	if len(got) != 1 || got[0].UserID != "peer-1" {
		t.Fatalf("unexpected cards: %+v", got)
	}
	if repo.filter != "" || repo.limit != 0 {
		t.Fatal("cache hit must not call repository")
	}
}

func TestGetDiscoverDeckCacheMissAndCampusFilter(t *testing.T) {
	srv, rdb := newMatchRedis(t)
	repo := &mockMatchRepository{
		vec:    pgvector.NewVector(make([]float32, 768)),
		campus: "ITB",
		cards:  []PeerCard{{UserID: "peer-1"}},
	}
	service := NewService(repo, rdb)

	got, err := service.GetDiscoverDeck(context.Background(), "u1", "campus")
	if err != nil {
		t.Fatalf("GetDiscoverDeck returned error: %v", err)
	}
	if len(got) != 1 || repo.filter != "ITB" || repo.limit != 20 {
		t.Fatalf("unexpected miss result: cards=%+v filter=%q limit=%d", got, repo.filter, repo.limit)
	}
	key := deckCacheKey("u1", "campus")
	if !srv.Exists(key) {
		t.Fatal("campus deck was not cached")
	}
	ttl, _ := rdb.TTL(context.Background(), key).Result()
	if ttl <= 14*time.Minute || ttl > 15*time.Minute {
		t.Fatalf("unexpected cache TTL: %v", ttl)
	}
}

func TestGetDiscoverDeckIncompleteProfile(t *testing.T) {
	_, rdb := newMatchRedis(t)
	repo := &mockMatchRepository{embeddingErr: ErrProfileIncomplete}
	service := NewService(repo, rdb)
	if _, err := service.GetDiscoverDeck(context.Background(), "u1", "all"); !errors.Is(err, ErrProfileIncomplete) && err == nil {
		t.Fatalf("expected ErrProfileIncomplete, got %v", err)
	}
}

func TestSwipeValidation(t *testing.T) {
	_, rdb := newMatchRedis(t)
	repo := &mockMatchRepository{}
	service := NewService(repo, rdb)

	if _, err := service.Swipe(context.Background(), "u1", SwipeInput{TargetUserID: "u1", Direction: "right"}); !errors.Is(err, ErrSelfSwipe) {
		t.Fatalf("expected ErrSelfSwipe, got %v", err)
	}
	if _, err := service.Swipe(context.Background(), "u1", SwipeInput{TargetUserID: "u2", Direction: "up"}); !errors.Is(err, ErrInvalidDirection) {
		t.Fatalf("expected ErrInvalidDirection, got %v", err)
	}
}

func TestSwipeLeftEvictsTargetFromBothCaches(t *testing.T) {
	_, rdb := newMatchRedis(t)
	ctx := context.Background()
	repo := &mockMatchRepository{swipeResult: &SwipeResult{Matched: false}}
	service := NewService(repo, rdb)

	raw, _ := json.Marshal([]PeerCard{{UserID: "u2"}, {UserID: "u3"}})
	_ = rdb.Set(ctx, deckCacheKey("u1", "all"), raw, time.Minute).Err()
	_ = rdb.Set(ctx, deckCacheKey("u1", "campus"), raw, time.Minute).Err()

	got, err := service.Swipe(ctx, "u1", SwipeInput{TargetUserID: "u2", Direction: "LEFT"})
	if err != nil {
		t.Fatalf("Swipe returned error: %v", err)
	}
	if got.Matched {
		t.Fatal("left swipe must not match")
	}
	for _, scope := range []string{"all", "campus"} {
		raw, err := rdb.Get(ctx, deckCacheKey("u1", scope)).Result()
		if err != nil {
			t.Fatalf("cache %s unexpectedly deleted or missing: %v", scope, err)
		}
		var cards []PeerCard
		_ = json.Unmarshal([]byte(raw), &cards)
		if len(cards) != 1 || cards[0].UserID != "u3" {
			t.Fatalf("target was not evicted from %s cache: %+v", scope, cards)
		}
	}
}

func TestSwipeRightMutualAndUnilateral(t *testing.T) {
	_, rdb := newMatchRedis(t)
	serviceMutual := NewService(&mockMatchRepository{swipeResult: &SwipeResult{
		Matched:     true,
		MatchID:     stringPointer("match-1"),
		WorkspaceID: stringPointer("workspace-1"),
	}}, rdb)
	got, err := serviceMutual.Swipe(context.Background(), "u1", SwipeInput{TargetUserID: "u2", Direction: "right"})
	if err != nil || !got.Matched || got.MatchID == nil || got.WorkspaceID == nil {
		t.Fatalf("unexpected mutual result: %+v, err=%v", got, err)
	}

	serviceSingle := NewService(&mockMatchRepository{swipeResult: &SwipeResult{Matched: false}}, rdb)
	got, err = serviceSingle.Swipe(context.Background(), "u1", SwipeInput{TargetUserID: "u3", Direction: "right"})
	if err != nil || got.Matched || got.MatchID != nil || got.WorkspaceID != nil {
		t.Fatalf("unexpected unilateral result: %+v, err=%v", got, err)
	}
}

func stringPointer(value string) *string { return &value }
