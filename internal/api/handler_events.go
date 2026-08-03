package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/adityabansal29/virtual-queue/internal/store"
)

// GetEvents handles GET /queue/events.
// Returns eventIds from Redis SCAN on queue:* keys.
// ponytail: SCAN over KEYS — safe on large keyspaces (D-09).
// ponytail: no auth — add bearer token or IP allowlist before production (T-02-05).
func (h *Handler) GetEvents(c *gin.Context) {
	ids, err := store.GetAllEventIDs(c.Request.Context(), h.rdb)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "redis scan failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"events": ids})
}
