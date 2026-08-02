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

// getPositionScript atomically returns the 0-based rank of ticketID in queueKey.
// Returns -1 if the ticket is not present (already admitted or expired).
const getPositionScript = `
local rank = redis.call('ZRANK', KEYS[1], ARGV[1])
if rank == false then return -1 end
return rank
`

// GetPosition returns the 0-based rank of ticketID in queueKey via a Lua script.
// Returns -1 if the ticket is not in the sorted set (admitted or expired).
func GetPosition(ctx context.Context, rdb *redis.Client, queueKey, ticketID string) (int64, error) {
	return rdb.Eval(ctx, getPositionScript, []string{queueKey}, ticketID).Int64()
}

// EventIDFromTicket returns the eventId stored in ticket:{ticketID} hash.
// Returns redis.Nil error if the ticket does not exist — caller should return 404.
func EventIDFromTicket(ctx context.Context, rdb *redis.Client, ticketID string) (string, error) {
	return rdb.HGet(ctx, "ticket:"+ticketID, "eventId").Result()
}
