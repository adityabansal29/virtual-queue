package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// QueueExit handles POST /queue/exit.
// Decrements active:{eventId} and returns 204. Called by the origin service
// when a user's session ends or checkout completes (QUEUE-09).
func (h *Handler) QueueExit(c *gin.Context) {
	var req struct {
		EventID string `json:"eventId"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	h.rdb.Decr(c.Request.Context(), "active:"+req.EventID) //nolint:errcheck
	c.Status(http.StatusNoContent)
}
