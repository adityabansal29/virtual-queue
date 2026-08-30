package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	internalaws "github.com/adityabansal29/virtual-queue/internal/aws"
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
	dw, err := internalaws.NewDynamoWriter(os.Getenv("DYNAMO_AUDIT_LOG_TABLE"))
	if err != nil {
		slog.Error("failed to init DynamoWriter", "error", err)
	}
	se, err := internalaws.NewSQSEmitter(os.Getenv("SQS_ADMISSION_QUEUE_URL"))
	if err != nil {
		slog.Error("failed to init SQSEmitter", "error", err)
	}
	sched := scheduler.NewScheduler(rdb, cfg, func(ticketID, eventID string) (string, error) {
		return token.IssueAdmission(ticketID, eventID, cfg.AdmissionSecret)
	}, dw, se)
	sched.Start(ctx)
}
