package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/adityabansal29/virtual-queue/internal/config"
	"github.com/adityabansal29/virtual-queue/internal/store"
	applog "github.com/adityabansal29/virtual-queue/pkg/log"
)

// Handler holds shared dependencies for all HTTP handlers.
type Handler struct {
	cfg       config.QueueServerConfig
	rdb       *redis.Client
	s3Client  *s3.Client
	s3Presign *s3.PresignClient
}

// NewHandler creates a Handler with the given config and Redis client.
func NewHandler(cfg config.QueueServerConfig, rdb *redis.Client, s3c *s3.Client) *Handler {
	var ps *s3.PresignClient
	if s3c != nil {
		ps = s3.NewPresignClient(s3c)
	}
	return &Handler{cfg: cfg, rdb: rdb, s3Client: s3c, s3Presign: ps}
}

// Join handles GET /queue/join?eventId=...&target=...
// Used by browser navigation (e.g. from QueueGuard error page link) and EW redirects.
// Resumes an existing queue position if q_ticket cookie is still in the sorted set;
// otherwise creates a new ticket.
func (h *Handler) Join(c *gin.Context) {
	eventID := c.Query("eventId")
	target := c.Query("target")
	if eventID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "eventId required"})
		return
	}

	ticketID, _ := c.Cookie("q_ticket")
	if !h.doesTicketExist(c, eventID, ticketID) {
		var err error
		ticketID, err = h.createTicket(c.Request.Context(), eventID)
		if err != nil {
			applog.ErrorWithContext(c.Request.Context(), "Join: createTicket failed", "eventId", eventID, "error", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "queue unavailable"})
			return
		}
		c.SetCookie("q_ticket", ticketID, config.QTicketCookieMaxAge, "/", "", false, true)
	}
	target = targetWithEventID(target, eventID)

	if h.s3Client != nil && h.cfg.QueuePageBucketName != "" {
		key := fmt.Sprintf("events/%s/page.html", eventID)
		if _, err := h.s3Client.HeadObject(c.Request.Context(), &s3.HeadObjectInput{Bucket: aws.String(h.cfg.QueuePageBucketName), Key: aws.String(key)}); err == nil {
			if pageURL, err := url.Parse(h.cfg.QueuePageURL); err == nil && pageURL.Scheme != "" && pageURL.Host != "" {
				pageURL.Path = "/" + key
				c.Redirect(http.StatusFound, pageURL.String())
				return
			}
		}
	}

	dest := fmt.Sprintf("%s?ticket=%s&target=%s",
		h.cfg.QueuePageURL, ticketID, url.QueryEscape(target))
	c.Redirect(http.StatusFound, dest)
}

// targetWithEventID carries event context through the waiting-page redirect so
// the stub-origin edge function can enforce the correct queue after admission.
func targetWithEventID(rawTarget, eventID string) string {
	if rawTarget == "" {
		return rawTarget
	}
	target, err := url.Parse(rawTarget)
	if err != nil {
		return rawTarget
	}
	query := target.Query()
	query.Set("eventId", eventID)
	target.RawQuery = query.Encode()
	return target.String()
}

// doesTicketExist reports whether ticketID still has a rank in the queue for this event.
func (h *Handler) doesTicketExist(c *gin.Context, eventID, ticketID string) bool {
	if ticketID == "" {
		return false
	}
	_, err := h.rdb.ZRank(c.Request.Context(), store.QueueKey(eventID), ticketID).Result()
	return err == nil
}

// createTicket writes the ticket to the sorted set and hash, returning the ticketID.
func (h *Handler) createTicket(ctx context.Context, eventID string) (string, error) {
	ticketID := uuid.New().String()
	score := float64(time.Now().UnixMilli())

	if err := h.rdb.ZAdd(ctx, store.QueueKey(eventID), redis.Z{
		Score:  score,
		Member: ticketID,
	}).Err(); err != nil {
		return "", err
	}

	// Non-fatal — status endpoint works via ZRank even if this fails.
	h.rdb.HSet(ctx, store.TicketKey(ticketID),
		"ticketId", ticketID,
		"eventId", eventID,
		"joinTime", score,
	) //nolint:errcheck
	h.rdb.Expire(ctx, store.TicketKey(ticketID), config.TicketKeyTTL) //nolint:errcheck

	return ticketID, nil
}
