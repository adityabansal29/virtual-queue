package main

import (
	"log/slog"

	"github.com/adityabansal29/virtual-queue/internal/api"
	"github.com/adityabansal29/virtual-queue/internal/config"
	"github.com/adityabansal29/virtual-queue/internal/store"
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

	rdb := store.NewQueueRedis(cfg.RedisAddr)
	handler := api.NewHandler(cfg, rdb)
	router := api.NewRouter(handler)

	if err := router.Run(":" + cfg.Port); err != nil {
		slog.Error("server failed", "error", err)
	}
}
