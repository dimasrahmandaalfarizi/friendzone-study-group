package auth

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrSessionNotFound = errors.New("session not found or expired")

type RedisSessionStore struct {
	client *redis.Client
	prefix string
}

func NewRedisSessionStore(client *redis.Client) *RedisSessionStore {
	return &RedisSessionStore{
		client: client,
		prefix: "session:",
	}
}

func (s *RedisSessionStore) Save(ctx context.Context, token, userID string, ttl time.Duration) error {
	return s.client.Set(ctx, s.prefix+token, userID, ttl).Err()
}

func (s *RedisSessionStore) GetUserID(ctx context.Context, token string) (string, error) {
	userID, err := s.client.Get(ctx, s.prefix+token).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrSessionNotFound
	}
	if err != nil {
		return "", err
	}
	return userID, nil
}

func (s *RedisSessionStore) Delete(ctx context.Context, token string) error {
	return s.client.Del(ctx, s.prefix+token).Err()
}