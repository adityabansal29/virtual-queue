package scheduler

import (
	"context"

	"github.com/adityabansal29/virtual-queue/internal/config"
	"github.com/redis/go-redis/v9"
)

// Scheduler runs admission ticks every second, popping tickets from the queue
// and issuing tokens via the injected issueToken function.
// Implementation provided by Plan 02; this file provides the API surface
// used by cmd/queueserver/main.go.
type Scheduler struct {
	rdb        *redis.Client
	cfg        config.Config
	issueToken func(ticketID, eventID string) (string, error)
}

// NewScheduler constructs a Scheduler with the given Redis client, config,
// and token issuer function. The issuer is called for each admitted ticket.
func NewScheduler(rdb *redis.Client, cfg config.Config, issueToken func(string, string) (string, error)) *Scheduler {
	return &Scheduler{rdb: rdb, cfg: cfg, issueToken: issueToken}
}

// Start runs the admission scheduler, ticking every second until ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	// ponytail: implementation filled in by Plan 02 (admission.go full impl);
	// this stub allows cmd/queueserver to compile before Plan 02 commits.
	<-ctx.Done()
}
