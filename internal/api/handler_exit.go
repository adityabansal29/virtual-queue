package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/adityabansal29/virtual-queue/internal/store"
)

// QueueExit handles POST /queue/exit.
// Decrements active session count and returns 204. Called by the origin when
// a checkout completes or session ends (QUEUE-09).
func (h *Handler) QueueExit(c *gin.Context) {
	var req struct {
		EventID string `json:"eventId"`
	}
	if err := c.BindJSON(&req); err != nil || req.EventID == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	h.rdb.Decr(c.Request.Context(), store.ActiveSessionCountKey(req.EventID)) //nolint:errcheck
	c.Status(http.StatusNoContent)
}
