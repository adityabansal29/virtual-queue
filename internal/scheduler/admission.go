package scheduler

import (
	"context"
	"fmt"
	"strconv"
	"time"

	jwtpkg "github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"

	internalaws "github.com/adityabansal29/virtual-queue/internal/aws"
	"github.com/adityabansal29/virtual-queue/internal/config"
	"github.com/adityabansal29/virtual-queue/internal/store"
	internaltoken "github.com/adityabansal29/virtual-queue/internal/token"
	applog "github.com/adityabansal29/virtual-queue/pkg/log"
)

// Scheduler ticks every second and admits tickets from the queue sorted set.
// issueToken is injected so the signing implementation can be swapped without
// changing this package.
type Scheduler struct {
	rdb        *redis.Client
	cfg        config.SchedulerConfig
	issueToken func(ticketID, eventID string) (string, error)
	dw         *internalaws.DynamoWriter
	se         *internalaws.SQSEmitter
}

// NewScheduler constructs a Scheduler with the given dependencies.
func NewScheduler(rdb *redis.Client, cfg config.SchedulerConfig, issueToken func(string, string) (string, error), dw *internalaws.DynamoWriter, se *internalaws.SQSEmitter) *Scheduler {
	return &Scheduler{rdb: rdb, cfg: cfg, issueToken: issueToken, dw: dw, se: se}
}

// Start runs the admission scheduler until ctx is cancelled.
// Tick interval is controlled by cfg.TickSecs (default 1s).
func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(s.cfg.TickSecs) * time.Second)
	defer ticker.Stop()
	applog.InfoWithContext(ctx, "scheduler started")
	for {
		select {
		case <-ticker.C:
			s.tickAllEvents(ctx)
		case <-ctx.Done():
			applog.InfoWithContext(ctx, "scheduler stopped")
			return
		}
	}
}

// tickAllEvents discovers active events via store.GetAllEventIDs and ticks each.
// ponytail: full SCAN per tick — add event registry when event count > 100.
func (s *Scheduler) tickAllEvents(ctx context.Context) {
	ids, err := store.GetAllEventIDs(ctx, s.rdb)
	if err != nil {
		applog.ErrorWithContext(ctx, "scheduler: scan failed", "error", err)
		return
	}
	for _, id := range ids {
		s.tick(ctx, id)
	}
}

func (s *Scheduler) tick(ctx context.Context, eventID string) {
	acquired, err := AcquireLock(ctx, s.rdb, eventID)
	if err != nil {
		applog.ErrorWithContext(ctx, "scheduler: lock error", "eventId", eventID, "error", err)
		return
	}
	if !acquired {
		return // another instance holds the lock — QUEUE-07
	}
	defer ReleaseLock(ctx, s.rdb, eventID) //nolint:errcheck

	rate, err := s.rdb.Get(ctx, store.AdmissionRateKey(eventID)).Int64()
	if err != nil {
		rate = s.cfg.DefaultAdmitRate
	}
	if rate == 0 {
		return
	}

	// D-06: admit min(rate, headroom); capacity=0 means unlimited.
	activeStr, _ := s.rdb.Get(ctx, store.ActiveSessionCountKey(eventID)).Result()
	active, _ := strconv.ParseInt(activeStr, 10, 64)
	capacityStr, _ := s.rdb.Get(ctx, store.MaxAllowedSessionCountKey(eventID)).Result()
	capacity, _ := strconv.ParseInt(capacityStr, 10, 64)

	var n int64
	if capacity > 0 {
		headroom := capacity - active
		if headroom <= 0 {
			return // T-02-12: zero headroom — skip ZPOPMIN entirely
		}
		n = min(rate, headroom)
	} else {
		n = rate
	}

	s.admitBatch(ctx, eventID, n)
}

func (s *Scheduler) admitBatch(ctx context.Context, eventID string, n int64) {
	members, err := s.rdb.ZPopMin(ctx, store.QueueKey(eventID), n).Result()
	if err != nil {
		applog.ErrorWithContext(ctx, "scheduler: zpopmin failed", "eventId", eventID, "error", err)
		return
	}
	if len(members) == 0 {
		return // QUEUE-06: empty queue is a no-op
	}

	for _, z := range members {
		ticketID, ok := z.Member.(string)
		if !ok {
			applog.WarnWithContext(ctx, "scheduler: unexpected member type", "member", z.Member)
			continue
		}

		jwt, err := s.issueToken(ticketID, eventID)
		if err != nil {
			applog.ErrorWithContext(ctx, "scheduler: issueToken failed", "ticketId", ticketID, "error", err)
			continue
		}

		if err := s.rdb.HSet(ctx, store.TicketKey(ticketID), "admission_token", jwt).Err(); err != nil {
			applog.ErrorWithContext(ctx, "scheduler: hset admission_token failed", "ticketId", ticketID, "error", err)
		} else {
			jti := ""
			parsed, parseErr := jwtpkg.ParseWithClaims(jwt, &internaltoken.AdmissionClaims{}, func(*jwtpkg.Token) (interface{}, error) {
				return []byte(s.cfg.AdmissionSecret), nil
			})
			if parseErr == nil {
				if claims, ok := parsed.Claims.(*internaltoken.AdmissionClaims); ok {
					jti = claims.ID
				}
			}
			if err := s.dw.Write(ctx, internalaws.SessionRecord{TicketID: ticketID, EventID: eventID, JTI: jti, AdmittedAt: time.Now()}); err != nil {
				applog.ErrorWithContext(ctx, "scheduler: dynamo write failed", "ticketId", ticketID, "error", err)
			}
			if err := s.se.Emit(ctx, internalaws.AdmissionEvent{TicketID: ticketID, EventID: eventID, JTI: jti}); err != nil {
				applog.ErrorWithContext(ctx, "scheduler: sqs emit failed", "ticketId", ticketID, "error", err)
			}
		}

		s.rdb.Incr(ctx, store.ActiveSessionCountKey(eventID)) //nolint:errcheck

		payload := fmt.Sprintf(`{"type":"admitted","token":"%s"}`, jwt)
		if err := s.rdb.Publish(ctx, store.TicketUpdatesKey(ticketID), payload).Err(); err != nil {
			applog.ErrorWithContext(ctx, "scheduler: publish admitted failed", "ticketId", ticketID, "error", err)
		}
	}

	applog.InfoWithContext(ctx, "scheduler: admitted batch", "eventId", eventID, "count", len(members))

	s.rdb.Publish(ctx, store.QueueTickKey(eventID), `{"type":"tick"}`) //nolint:errcheck
}
