package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/adityabansal29/virtual-queue/internal/store"
	"github.com/adityabansal29/virtual-queue/internal/token"
)

// ValidateAdmission validates an admission JWT and its ticket binding without
// consuming it. One-time redemption remains the origin's responsibility, so
// retries can safely call this endpoint before origin-side SETNX.
func (h *Handler) ValidateAdmission(c *gin.Context) {
	var req struct {
		TicketID string `json:"ticketID"`
		Token    string `json:"token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.TicketID == "" || req.Token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ticketID and token required"})
		return
	}

	claims, err := token.ValidateJWT(req.Token, h.cfg.AdmissionSecret)
	if err != nil || claims.TicketID != req.TicketID || claims.Subject != req.TicketID {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	eventID, err := store.EventIDFromTicket(c.Request.Context(), h.rdb, req.TicketID)
	if err != nil || eventID != claims.EventID {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	c.JSON(http.StatusOK, gin.H{"eventID": eventID, "ticketID": req.TicketID, "jti": claims.ID})
}
