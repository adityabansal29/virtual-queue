package store

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewQueueRedis creates and returns a Redis client for the queue service.
// Pings Redis on creation; logs failure but returns the client anyway
// so Docker health checks handle retry.
// ponytail: no wrapper struct — add one when multiple ops need transactional grouping.
func NewQueueRedis(addr string) *redis.Client {
	client := redis.NewClient(&redis.Options{Addr: addr})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		slog.Error("redis ping failed", "addr", addr, "error", err)
	}

	return client
}
