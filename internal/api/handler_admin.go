package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"

	"github.com/adityabansal29/virtual-queue/internal/store"
)

func (h *Handler) GetPageUploadURL(c *gin.Context) {
	if h.s3Presign == nil || h.cfg.EventsBucketName == "" {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "S3 not configured"})
		return
	}
	eventID := c.Param("id")
	if eventID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "eventId required"})
		return
	}
	key := fmt.Sprintf("events/%s/page.html", eventID)
	req, err := h.s3Presign.PresignPutObject(c.Request.Context(), &s3.PutObjectInput{Bucket: aws.String(h.cfg.EventsBucketName), Key: aws.String(key), ContentType: aws.String("text/html")}, s3.WithPresignExpires(15*time.Minute))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "presign failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": req.URL})
}

// UpdateRate handles PUT /queue/rate/:eventId.
// Sets rate and optional capacity in Redis.
// ponytail: no auth — add bearer token or IP allowlist before production (T-04-01).
func (h *Handler) UpdateRate(c *gin.Context) {
	eventID := c.Param("eventId")
	ctx := c.Request.Context()

	var req struct {
		Rate     int64 `json:"rate"`
		Capacity int64 `json:"capacity,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.Rate > 0 {
		h.rdb.Set(ctx, store.AdmissionRateKey(eventID), req.Rate, 0)
	}
	if req.Capacity > 0 {
		h.rdb.Set(ctx, store.MaxAllowedSessionCountKey(eventID), req.Capacity, 0)
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
	ctx := c.Request.Context()

	depth, _ := h.rdb.ZCard(ctx, store.QueueKey(eventID)).Result()

	activeStr, _ := h.rdb.Get(ctx, store.ActiveSessionCountKey(eventID)).Result()
	active, _ := strconv.ParseInt(activeStr, 10, 64)

	rateStr, _ := h.rdb.Get(ctx, store.AdmissionRateKey(eventID)).Result()
	rate, err := strconv.ParseInt(rateStr, 10, 64)
	if err != nil || rate == 0 {
		rate = h.cfg.DefaultAdmitRate
	}

	capacityStr, _ := h.rdb.Get(ctx, store.MaxAllowedSessionCountKey(eventID)).Result()
	capacity, _ := strconv.ParseInt(capacityStr, 10, 64)

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
