package match

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pgvector/pgvector-go"
	"github.com/redis/go-redis/v9"
)

const (
	deckBatchSize = 20
	deckCacheTTL  = 15 * time.Minute
)

type Repository interface {
	GetUserEmbeddingAndCampus(ctx context.Context, userID string) (pgvector.Vector, string, error)
	FetchDiscoveryDeck(ctx context.Context, currentUserID string, userVec pgvector.Vector, universityFilter string, limit int) ([]PeerCard, error)
	RecordSwipe(ctx context.Context, swiperID, targetID, direction string) (*SwipeResult, error)
	ListMutualMatches(ctx context.Context, userID string) ([]MutualMatch, error)
}

type Service struct {
	repo Repository
	rdb  *redis.Client
}

func NewService(repo Repository, rdb *redis.Client) *Service {
	return &Service{
		repo: repo,
		rdb:  rdb,
	}
}

func deckCacheKey(userID, scope string) string {
	return fmt.Sprintf("discover:deck:%s:%s", userID, scope)
}

func (s *Service) GetDiscoverDeck(ctx context.Context, userID, rawScope string) ([]PeerCard, error) {
	scope := "all"
	if strings.ToLower(strings.TrimSpace(rawScope)) == "campus" {
		scope = "campus"
	}

	key := deckCacheKey(userID, scope)

	// 1. Cek cache Redis terlebih dahulu
	if cached, err := s.rdb.Get(ctx, key).Result(); err == nil && cached != "" {
		var cards []PeerCard
		if err := json.Unmarshal([]byte(cached), &cards); err == nil && len(cards) > 0 {
			return cards, nil
		}
	}

	// 2. Cache miss / deck habis: ambil batch baru dari PostgreSQL pgvector
	vec, university, err := s.repo.GetUserEmbeddingAndCampus(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load current user's embedding and campus: %w", err)
	}

	universityFilter := ""
	if scope == "campus" {
		universityFilter = university
	}

	cards, err := s.repo.FetchDiscoveryDeck(ctx, userID, vec, universityFilter, deckBatchSize)
	if err != nil {
		return nil, fmt.Errorf("fetch discovery candidates: %w", err)
	}

	// Simpan ke Redis jika ada hasil
	if len(cards) > 0 {
		if raw, err := json.Marshal(cards); err == nil {
			_ = s.rdb.Set(ctx, key, raw, deckCacheTTL).Err()
		}
	}

	return cards, nil
}

func (s *Service) Swipe(ctx context.Context, swiperID string, input SwipeInput) (*SwipeResult, error) {
	targetID := strings.TrimSpace(input.TargetUserID)
	direction := strings.ToLower(strings.TrimSpace(input.Direction))

	if targetID == "" || swiperID == targetID {
		return nil, ErrSelfSwipe
	}
	if direction != "left" && direction != "right" {
		return nil, ErrInvalidDirection
	}

	result, err := s.repo.RecordSwipe(ctx, swiperID, targetID, direction)
	if err != nil {
		return nil, err
	}

	// Keluarkan kartu yang baru di-swipe dari cache Redis
	s.evictCardFromCache(ctx, swiperID, targetID, "all")
	s.evictCardFromCache(ctx, swiperID, targetID, "campus")

	return result, nil
}

func (s *Service) evictCardFromCache(ctx context.Context, userID, swipedTargetID, scope string) {
	key := deckCacheKey(userID, scope)
	cached, err := s.rdb.Get(ctx, key).Result()
	if err != nil || cached == "" {
		return
	}

	var cards []PeerCard
	if err := json.Unmarshal([]byte(cached), &cards); err != nil {
		_ = s.rdb.Del(ctx, key).Err()
		return
	}

	filtered := make([]PeerCard, 0, len(cards))
	for _, c := range cards {
		if c.UserID != swipedTargetID {
			filtered = append(filtered, c)
		}
	}

	if len(filtered) == 0 {
		_ = s.rdb.Del(ctx, key).Err()
		return
	}

	if raw, err := json.Marshal(filtered); err == nil {
		_ = s.rdb.Set(ctx, key, raw, deckCacheTTL).Err()
	}
}

func (s *Service) GetMutualMatches(ctx context.Context, userID string) ([]MutualMatch, error) {
	return s.repo.ListMutualMatches(ctx, userID)
}
