package store

import (
	"context"
	"crypto/tls"
	"os"
	"time"

	"github.com/redis/go-redis/v9"

	applog "github.com/adityabansal29/virtual-queue/pkg/log"
)

// NewQueueRedis creates and returns a Redis client for the queue service.
// Pings Redis on creation; logs failure but returns the client anyway
// so Docker health checks handle retry.
// ponytail: no wrapper struct — add one when multiple ops need transactional grouping.
func NewQueueRedis(addr string) *redis.Client {
	options := &redis.Options{Addr: addr}
	if os.Getenv("REDIS_TLS") == "true" {
		options.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	client := redis.NewClient(options)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		applog.ErrorWithContext(ctx, "redis ping failed", "addr", addr, "error", err)
	}

	return client
}

// GetPosition returns the 0-based rank of ticketID in queueKey.
// Returns -1, nil if the ticket is not in the sorted set (admitted or expired).
func GetPosition(ctx context.Context, rdb *redis.Client, queueKey, ticketID string) (int64, error) {
	rank, err := rdb.ZRank(ctx, queueKey, ticketID).Result()
	if err == redis.Nil {
		return -1, nil
	}
	return rank, err
}

// EventIDFromTicket returns the eventId stored in the ticket hash.
// Returns redis.Nil error if the ticket does not exist — caller should return 404.
func EventIDFromTicket(ctx context.Context, rdb *redis.Client, ticketID string) (string, error) {
	return rdb.HGet(ctx, TicketKey(ticketID), "eventId").Result()
}
