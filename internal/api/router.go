package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// NewRouter wires all handlers to a Gin engine and returns it.
func NewRouter(h *Handler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// Implemented endpoints
	r.POST("/queue/join", h.Join)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Placeholder endpoints — implemented in later plans.
	stub := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "not implemented"})
	}
	r.GET("/queue/status/:ticketId", stub)
	r.POST("/queue/exit", stub)
	r.PUT("/queue/rate/:eventId", stub)
	r.GET("/queue/config/:eventId", stub)

	return r
}
