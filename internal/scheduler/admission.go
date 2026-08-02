package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/adityabansal29/virtual-queue/internal/config"
)

// Scheduler ticks every second and admits tickets from the queue sorted set.
// issueToken is injected so Plan 03 can replace the stub with a real JWT issuer
// without changing this package.
// TODO Plan 03: replace stub issueToken with token.IssueAdmission(ticketID, eventID, cfg.AdmissionSecret)
type Scheduler struct {
	rdb        *redis.Client
	cfg        config.Config
	issueToken func(ticketID, eventID string) (string, error)
}

// NewScheduler constructs a Scheduler with the given dependencies.
func NewScheduler(rdb *redis.Client, cfg config.Config, issueToken func(string, string) (string, error)) *Scheduler {
	return &Scheduler{rdb: rdb, cfg: cfg, issueToken: issueToken}
}

// Start runs the admission scheduler. Ticks every second until ctx is cancelled.
// Call as a goroutine: go sched.Start(ctx).
func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	slog.Info("scheduler started", "eventId", s.cfg.DefaultEventID)
	for {
		select {
		case <-ticker.C:
			s.tick(ctx, s.cfg.DefaultEventID)
		case <-ctx.Done():
			slog.Info("scheduler stopped")
			return
		}
	}
}

func (s *Scheduler) tick(ctx context.Context, eventID string) {
	acquired, err := AcquireLock(ctx, s.rdb, eventID)
	if err != nil {
		slog.Error("scheduler: lock error", "eventId", eventID, "error", err)
		return
	}
	if !acquired {
		return // another instance holds the lock — QUEUE-07
	}
	defer ReleaseLock(ctx, s.rdb, eventID) //nolint:errcheck

	// Read configured rate; fall back to default if key absent.
	rate, err := s.rdb.Get(ctx, "rate:"+eventID).Int64()
	if err != nil {
		rate = s.cfg.DefaultAdmitRate
	}
	if rate == 0 {
		return // D-06: rate == 0 means skip ZPOPMIN entirely
	}

	// D-06 (Phase 2): admit min(rate, headroom); headroom = capacity - active. capacity=0 means unlimited.
	activeStr, _ := s.rdb.Get(ctx, "active:"+eventID).Result()
	active, _ := strconv.ParseInt(activeStr, 10, 64)
	capacityStr, _ := s.rdb.Get(ctx, "capacity:"+eventID).Result()
	capacity, _ := strconv.ParseInt(capacityStr, 10, 64)

	var n int64
	if capacity > 0 {
		headroom := capacity - active
		if headroom <= 0 {
			return // T-02-12: zero headroom — skip ZPOPMIN entirely
		}
		n = min(rate, headroom)
	} else {
		n = rate // capacity=0 means unconfigured/unlimited
	}

	s.admitBatch(ctx, eventID, n)
}

func (s *Scheduler) admitBatch(ctx context.Context, eventID string, n int64) {
	members, err := s.rdb.ZPopMin(ctx, "queue:"+eventID, n).Result()
	if err != nil {
		slog.Error("scheduler: zpopmin failed", "eventId", eventID, "error", err)
		return
	}
	if len(members) == 0 {
		// QUEUE-06: empty queue is a no-op — ZPOPMIN on empty set returns empty slice, not error.
		return
	}

	for _, z := range members {
		ticketID, ok := z.Member.(string)
		if !ok {
			slog.Warn("scheduler: unexpected member type", "member", z.Member)
			continue
		}

		jwt, err := s.issueToken(ticketID, eventID)
		if err != nil {
			slog.Error("scheduler: issueToken failed", "ticketId", ticketID, "error", err)
			continue
		}

		// QUEUE-08: store token in ticket hash so poll tier can pick it up.
		if err := s.rdb.HSet(ctx, "ticket:"+ticketID, "admission_token", jwt).Err(); err != nil {
			slog.Error("scheduler: hset admission_token failed", "ticketId", ticketID, "error", err)
		}

		// TOKEN-07 scaffolding: increment active count (not enforced as ceiling per D-06).
		s.rdb.Incr(ctx, "active:"+eventID) //nolint:errcheck

		// Publish admission event to per-ticket channel for SSE subscribers.
		payload := fmt.Sprintf(`{"type":"admitted","token":"%s"}`, jwt)
		if err := s.rdb.Publish(ctx, "ticket:updates:"+ticketID, payload).Err(); err != nil {
			slog.Error("scheduler: publish admitted failed", "ticketId", ticketID, "error", err)
		}
	}

	slog.Info("scheduler: admitted batch", "eventId", eventID, "count", len(members))

	// Notify SSE handlers subscribed to the queue tick channel to re-read their rank.
	s.rdb.Publish(ctx, "queue:tick:"+eventID, `{"type":"tick"}`) //nolint:errcheck
}
