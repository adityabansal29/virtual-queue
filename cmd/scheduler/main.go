package main

import (
	"context"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/adityabansal29/virtual-queue/internal/config"
	"github.com/adityabansal29/virtual-queue/internal/scheduler"
	"github.com/adityabansal29/virtual-queue/internal/store"
	"github.com/adityabansal29/virtual-queue/internal/token"
)

func main() {
	cfg := config.LoadScheduler()

	slog.Info("scheduler starting",
		"redis", cfg.RedisAddr,
		"admission_secret", "set",
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	rdb := store.NewQueueRedis(cfg.RedisAddr)
	sched := scheduler.NewScheduler(rdb, cfg, func(ticketID, eventID string) (string, error) {
		return token.IssueAdmission(ticketID, eventID, cfg.AdmissionSecret)
	})
	sched.Start(ctx)
}
