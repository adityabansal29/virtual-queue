package scheduler

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/adityabansal29/virtual-queue/internal/store"
)

// AcquireLock attempts to acquire the distributed leader lock for eventID via SETNX.
// Returns (true, nil) if acquired, (false, nil) if already held, (false, err) on Redis failure.
// ponytail: per-event lock is a failsafe for restart-overlap races — cheap when only one instance runs.
func AcquireLock(ctx context.Context, rdb *redis.Client, eventID string) (bool, error) {
	return rdb.SetNX(ctx, store.SchedulerLockKey(eventID), "1", 10*time.Second).Result()
}

// ReleaseLock releases the distributed leader lock for eventID.
func ReleaseLock(ctx context.Context, rdb *redis.Client, eventID string) error {
	return rdb.Del(ctx, store.SchedulerLockKey(eventID)).Err()
}
