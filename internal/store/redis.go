package store

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisStore(redisAddr string) *RedisStore {
	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	return &RedisStore{
		client: client,
		ctx:    context.Background(),
	}
}

func (s *RedisStore) AddURL(shortCode string, url string) error {
	return s.client.Set(s.ctx, shortCode, url, 0).Err()
}

func (s *RedisStore) GetURL(shortCode string) (string, bool) {
	value, err := s.client.Get(s.ctx, shortCode).Result()
	if err == redis.Nil {
		return "", false
	}
	if err != nil {
		return "", false
	}
	return value, true
}
