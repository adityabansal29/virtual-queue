package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetEvents handles GET /queue/events.
// Returns eventIds from Redis SCAN on queue:* keys.
// ponytail: SCAN over KEYS — safe on large keyspaces (D-09).
// ponytail: no auth — add bearer token or IP allowlist before production (T-02-05).
func (h *Handler) GetEvents(c *gin.Context) {
	ctx := context.Background()
	var cursor uint64
	var eventIDs []string
	for {
		keys, next, err := h.rdb.Scan(ctx, cursor, "queue:*", 100).Result()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "redis scan failed"})
			return
		}
		for _, k := range keys {
			eventIDs = append(eventIDs, k[len("queue:"):]) // strip "queue:" prefix
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	if eventIDs == nil {
		eventIDs = []string{} // never return null — admin JS expects array
	}
	c.JSON(http.StatusOK, gin.H{"events": eventIDs})
}
