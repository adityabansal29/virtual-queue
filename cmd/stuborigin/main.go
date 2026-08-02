package main

import (
	"log/slog"
	"net/http"

	"github.com/adityabansal29/virtual-queue/internal/config"
	"github.com/adityabansal29/virtual-queue/internal/store"
	"github.com/adityabansal29/virtual-queue/pkg/middleware"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	slog.Info("stub origin starting",
		"port", "8081",
		"redis", cfg.RedisAddr,
		// T-01-05: never log secret values.
		"admission_secret", "set",
		"session_secret", "set",
	)

	// redis-origin client — REDIS_ADDR is set to redis-origin:6379 in Docker Compose.
	// D-02/D-03: stub origin uses redis-origin, NOT redis-queue.
	originRedis := store.NewQueueRedis(cfg.RedisAddr)

	mwCfg := middleware.Config{
		AdmissionSecret: cfg.AdmissionSecret,
		SessionSecret:   cfg.SessionSecret,
		QueueURL:        "http://localhost:8080",
		RDB:             originRedis,
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// ponytail: static seat selection handler — Phase 2 extracts session claims
	// from context key when real seat display is needed.
	r.GET("/", middleware.QueueGuard(mwCfg), func(c *gin.Context) {
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK,
			"<html><body><h1>Seat Selection</h1><p>Admitted — enjoy the event!</p></body></html>")
	})

	if err := r.Run(":8081"); err != nil {
		slog.Error("stub origin failed", "error", err)
	}
}
