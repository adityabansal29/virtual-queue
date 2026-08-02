package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// UpdateRate handles PUT /queue/rate/:eventId.
// Sets rate:{eventId} (and optional capacity:{eventId}) in Redis.
// ponytail: no auth — add bearer token or IP allowlist before production (T-04-01).
func (h *Handler) UpdateRate(c *gin.Context) {
	eventID := c.Param("eventId")

	var req struct {
		Rate     int64 `json:"rate"`
		Capacity int64 `json:"capacity,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	ctx := context.Background()

	if req.Rate > 0 {
		// No TTL — rate persists until changed. Scheduler reads on next tick.
		h.rdb.Set(ctx, "rate:"+eventID, req.Rate, 0)
	}
	if req.Capacity > 0 {
		// Scaffolded for D-06: capacity:{eventId} stored but not enforced in Phase 1.
		h.rdb.Set(ctx, "capacity:"+eventID, req.Capacity, 0)
	}

	c.JSON(http.StatusOK, gin.H{
		"eventId":  eventID,
		"rate":     req.Rate,
		"capacity": req.Capacity,
		"message":  "rate updated; takes effect on next scheduler tick",
	})
}

// GetConfig handles GET /queue/config/:eventId.
// Returns queue depth, active users, admit rate, capacity, and estimated drain time.
func (h *Handler) GetConfig(c *gin.Context) {
	eventID := c.Param("eventId")
	ctx := context.Background()

	depth, _ := h.rdb.ZCard(ctx, "queue:"+eventID).Result()

	activeStr, _ := h.rdb.Get(ctx, "active:"+eventID).Result()
	active, _ := strconv.ParseInt(activeStr, 10, 64)

	rateStr, _ := h.rdb.Get(ctx, "rate:"+eventID).Result()
	rate, err := strconv.ParseInt(rateStr, 10, 64)
	if err != nil || rate == 0 {
		rate = h.cfg.DefaultAdmitRate
	}

	capacityStr, _ := h.rdb.Get(ctx, "capacity:"+eventID).Result()
	capacity, _ := strconv.ParseInt(capacityStr, 10, 64)
	// D-06: capacity not enforced in Phase 1; stored for future use.

	// Integer division — sufficient for Phase 1 drain estimate.
	estimatedSec := int64(-1)
	if rate > 0 {
		estimatedSec = depth / rate * 60
	}

	c.JSON(http.StatusOK, gin.H{
		"eventId":           eventID,
		"queueDepth":        depth,
		"activeUsers":       active,
		"admitRate":         rate,
		"capacity":          capacity,
		"estimatedDrainSec": estimatedSec,
	})
}
