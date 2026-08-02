package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/adityabansal29/virtual-queue/internal/config"
)

// Handler holds shared dependencies for all HTTP handlers.
type Handler struct {
	cfg config.Config
	rdb *redis.Client
}

// NewHandler creates a Handler with the given config and Redis client.
func NewHandler(cfg config.Config, rdb *redis.Client) *Handler {
	return &Handler{cfg: cfg, rdb: rdb}
}

// Join handles POST /queue/join.
// Generates a ticketId, ZADDs it to queue:{eventId}, and stores a ticket hash.
func (h *Handler) Join(c *gin.Context) {
	var req struct {
		EventID string `json:"eventId"`
	}
	// Non-fatal parse: missing body defaults to DefaultEventID.
	_ = c.ShouldBindJSON(&req)
	if req.EventID == "" {
		req.EventID = h.cfg.DefaultEventID
	}

	ticketID := uuid.New().String()
	score := float64(time.Now().UnixMilli())
	ctx := context.Background()

	// ZADD queue:{eventId} <score> <ticketId>
	// ponytail: ZADD without NX — UUID collision probability ~0;
	// add ZAddArgs{NX:true} if idempotent rejoin semantics are required.
	if err := h.rdb.ZAdd(ctx, "queue:"+req.EventID, redis.Z{
		Score:  score,
		Member: ticketID,
	}).Err(); err != nil {
		slog.Error("zadd failed", "eventId", req.EventID, "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "queue unavailable"})
		return
	}

	// Store ticket metadata in hash ticket:{ticketId}
	if err := h.rdb.HSet(ctx, "ticket:"+ticketID,
		"ticketId", ticketID,
		"eventId", req.EventID,
		"joinTime", score,
	).Err(); err != nil {
		slog.Error("hset failed", "ticketId", ticketID, "error", err)
		// HSet failure is non-fatal for the join response; ticket is already in queue.
		// Log and continue — status endpoint will still work via ZRANK.
	}

	c.JSON(http.StatusOK, gin.H{
		"ticketId": ticketID,
		"eventId":  req.EventID,
		"position": "queued",
	})
}
