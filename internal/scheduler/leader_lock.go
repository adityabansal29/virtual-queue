package scheduler

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// AcquireLock attempts to acquire the distributed leader lock for eventID via SETNX.
// Returns (true, nil) if the lock was acquired, (false, nil) if already held by another
// instance, and (false, err) on Redis failure.
func AcquireLock(ctx context.Context, rdb *redis.Client, eventID string) (bool, error) {
	return rdb.SetNX(ctx, "scheduler:lock:"+eventID, "1", 10*time.Second).Result()
}

// ReleaseLock releases the distributed leader lock for eventID.
func ReleaseLock(ctx context.Context, rdb *redis.Client, eventID string) error {
	return rdb.Del(ctx, "scheduler:lock:"+eventID).Err()
}
