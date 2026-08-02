package main

import (
	"context"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/adityabansal29/virtual-queue/internal/api"
	"github.com/adityabansal29/virtual-queue/internal/config"
	"github.com/adityabansal29/virtual-queue/internal/scheduler"
	"github.com/adityabansal29/virtual-queue/internal/store"
	"github.com/adityabansal29/virtual-queue/internal/token"
)

func main() {
	cfg := config.Load()

	slog.Info("queue server starting",
		"port", cfg.Port,
		"redis", cfg.RedisAddr,
		// T-01-05: never log secret values — only log presence.
		"admission_secret", "set",
		"session_secret", "set",
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	rdb := store.NewQueueRedis(cfg.RedisAddr)
	handler := api.NewHandler(cfg, rdb)
	router := api.NewRouter(handler)

	// Wire scheduler with the real JWT issuer (Plan 03).
	// IssueAdmission signs with ADMISSION_SECRET — returns a UUID-JTI-based HMAC-SHA256 JWT.
	sched := scheduler.NewScheduler(rdb, cfg, func(ticketID, eventID string) (string, error) {
		return token.IssueAdmission(ticketID, eventID, cfg.AdmissionSecret)
	})
	go sched.Start(ctx)

	if err := router.Run(":" + cfg.Port); err != nil {
		slog.Error("server failed", "error", err)
	}
}
