package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// NewRouter wires all handlers to a Gin engine and returns it.
func NewRouter(h *Handler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// ponytail: allowlist only; extend slice if static-pages port changes (D-03).
	r.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowed := []string{"http://localhost:8082", "http://localhost:8081"}
		for _, o := range allowed {
			if origin == o {
				c.Header("Access-Control-Allow-Origin", o)
				break
			}
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// Implemented endpoints
	r.POST("/queue/join", h.Join)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Status: poll or SSE depending on ?mode= query param (ponytail: extract if more modes added).
	r.GET("/queue/status/:ticketId", func(c *gin.Context) {
		if c.Query("mode") == "sse" {
			h.QueueStatusSSE(c)
		} else {
			h.QueueStatusPoll(c)
		}
	})

	r.POST("/queue/exit", h.QueueExit)

	r.PUT("/queue/rate/:eventId", h.UpdateRate)
	r.GET("/queue/config/:eventId", h.GetConfig)
	r.GET("/queue/events", h.GetEvents)

	return r
}
