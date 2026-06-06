package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisSession struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisSession(URL string, ttl time.Duration) (*RedisSession, error) {
	opt, err := redis.ParseURL(URL)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(opt)
	return &RedisSession{
		client: client,
		ttl:    ttl,
	}, nil
}
func (s *RedisSession) PingRedisClient() error {
	return s.client.Ping(context.Background()).Err()
}
func (s *RedisSession) AddIfNotExists(ctx context.Context, id string) (bool, error) {
	return s.client.SetNX(ctx, id, "1", s.ttl).Result()
}
