package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/adityabansal29/virtual-queue/internal/store"
)

// QueueStatusPoll handles GET /queue/status/:ticketId?mode=poll (or no mode param).
// Single-shot, stateless — client re-calls every 5s.
func (h *Handler) QueueStatusPoll(c *gin.Context) {
	ticketID := c.Param("ticketId")
	ctx := c.Request.Context()

	eventID, err := store.EventIDFromTicket(ctx, h.rdb, ticketID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ticket not found"})
		return
	}

	// Check if already admitted: token written by scheduler to ticket hash (QUEUE-08).
	// Read-once: delete immediately so a second poll doesn't double-deliver.
	if token, err := h.rdb.HGet(ctx, "ticket:"+ticketID, "admission_token").Result(); err == nil {
		h.rdb.HDel(ctx, "ticket:"+ticketID, "admission_token")
		c.JSON(http.StatusOK, gin.H{"type": "admitted", "token": token})
		return
	}

	rank, err := store.GetPosition(ctx, h.rdb, "queue:"+eventID, ticketID)
	if err != nil || rank < 0 {
		// Ticket popped from sorted set but token not yet written — transient window.
		c.JSON(http.StatusOK, gin.H{"type": "pending"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"type":          "position",
		"value":         rank,
		"upgrade_to_sse": rank < int64(h.cfg.SSEThreshold),
	})
}

// QueueStatusSSE handles GET /queue/status/:ticketId?mode=sse.
// Opens a persistent SSE stream. Subscription established BEFORE initial rank read
// to close the race window where admission occurs between rank read and subscribe (QUEUE-05).
func (h *Handler) QueueStatusSSE(c *gin.Context) {
	ticketID := c.Param("ticketId")
	ctx := c.Request.Context()

	eventID, err := store.EventIDFromTicket(ctx, h.rdb, ticketID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ticket not found"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		// Should never happen with standard Gin ResponseWriter.
		panic("QueueStatusSSE: gin.ResponseWriter does not implement http.Flusher")
	}

	// QUEUE-05: subscribe BEFORE reading initial rank so no admission event is missed.
	pubsub := h.rdb.Subscribe(ctx,
		"queue:tick:"+eventID,
		"ticket:updates:"+ticketID,
	)
	defer pubsub.Close()

	// Send initial position event if ticket is still in the queue.
	if rank, err := store.GetPosition(ctx, h.rdb, "queue:"+eventID, ticketID); err == nil && rank >= 0 {
		fmt.Fprintf(c.Writer, "event: update\ndata: {\"type\":\"position\",\"value\":%d}\n\n", rank)
		flusher.Flush()
	}

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case msg := <-pubsub.Channel():
			var ev map[string]string
			if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
				slog.Warn("sse: failed to unmarshal pub/sub payload", "payload", msg.Payload, "error", err)
				continue
			}

			switch ev["type"] {
			case "tick":
				rank, err := store.GetPosition(ctx, h.rdb, "queue:"+eventID, ticketID)
				if err != nil || rank < 0 {
					// Ticket popped — stay in the loop and wait for the admitted pub/sub message.
					slog.Debug("sse: ticket popped, awaiting admitted event", "ticketId", ticketID)
					continue
				}
				data, _ := json.Marshal(map[string]any{"type": "position", "value": rank})
				fmt.Fprintf(c.Writer, "event: update\ndata: %s\n\n", data)
				flusher.Flush()

			case "admitted":
				data, _ := json.Marshal(map[string]string{
					"type":  "admitted",
					"token": ev["token"],
				})
				fmt.Fprintf(c.Writer, "event: update\ndata: %s\n\n", data)
				flusher.Flush()
				return
			}

		case <-heartbeat.C:
			fmt.Fprintf(c.Writer, ": heartbeat\n\n")
			flusher.Flush()

		case <-ctx.Done():
			return
		}
	}
}
