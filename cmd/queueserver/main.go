package main

import (
	"context"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/adityabansal29/virtual-queue/internal/api"
	"github.com/adityabansal29/virtual-queue/internal/config"
	"github.com/adityabansal29/virtual-queue/internal/store"
)

func main() {
	cfg := config.LoadQueueServer()

	slog.Info("queue server starting",
		"port", cfg.Port,
		"redis", cfg.RedisAddr,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	_ = ctx // reserved for graceful shutdown wiring

	rdb := store.NewQueueRedis(cfg.RedisAddr)
	var s3Client *s3.Client
	if cfg.QueuePageBucketName != "" {
		if awsCfg, err := awsconfig.LoadDefaultConfig(context.Background()); err == nil {
			s3Client = s3.NewFromConfig(awsCfg)
		}
	}
	handler := api.NewHandler(cfg, rdb, s3Client)
	router := api.NewRouter(handler)

	if err := router.Run(":" + cfg.Port); err != nil {
		slog.Error("server failed", "error", err)
	}
}
