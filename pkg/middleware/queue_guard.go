package middleware

import (
	"net/http"
	"net/url"
	"time"

	"github.com/adityabansal29/virtual-queue/internal/token"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// Config holds the parameters for the QueueGuard middleware.
// RDB must point to the redis-origin client (D-03), NOT redis-queue.
type Config struct {
	AdmissionSecret string
	SessionSecret   string
	QueueURL        string
	RDB             *redis.Client
}

// QueueGuard returns a Gin middleware that enforces the two-cookie token model
// (DESIGN.md Section 8). The exact flow:
//  1. Session fast-path: valid q_session → pass through.
//  2. No q_admission → redirect to queue.
//  3. Invalid admission JWT → redirect to queue.
//  4. SETNX token:{jti} — atomic one-time use (TOKEN-04). Failure → 403.
//  5. Increment active:{eventId}.
//  6. Issue q_session signed with SessionSecret.
//  7. Clear q_admission.
//  8. Pass to handler.
func QueueGuard(cfg Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Session cookie fast-path — verified with SESSION_SECRET.
		if sc, err := c.Cookie("q_session"); err == nil {
			if _, err := token.ValidateSession(sc, cfg.SessionSecret); err == nil {
				c.Next()
				return
			}
		}

		// 2. Admission token required.
		ac, err := c.Cookie("q_admission")
		if err != nil {
			target := cfg.QueueURL + "?target=" + url.QueryEscape(c.Request.URL.String())
			c.Redirect(http.StatusFound, target)
			c.Abort()
			return
		}

		// 3. Verify JWT signature and expiry against ADMISSION_SECRET.
		claims, err := token.ValidateJWT(ac, cfg.AdmissionSecret)
		if err != nil {
			target := cfg.QueueURL + "?target=" + url.QueryEscape(c.Request.URL.String())
			c.Redirect(http.StatusFound, target)
			c.Abort()
			return
		}

		// 4. SETNX — atomic one-time enforcement (TOKEN-04).
		// Redis SETNX guarantees: even if two requests with identical JWTs arrive
		// simultaneously, exactly one Set call returns true. The other returns false
		// and gets 403. No race condition possible.
		set, err := cfg.RDB.SetNX(c.Request.Context(), "token:"+claims.ID, "used", 30*time.Minute).Result()
		if err != nil || !set {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		// 5. Increment active count (TOKEN-05, capacity accounting for Phase 2).
		cfg.RDB.Incr(c.Request.Context(), "active:"+claims.EventID)

		// 6. Issue q_session cookie signed with SESSION_SECRET.
		sc, _ := token.IssueSession(claims.Subject, claims.EventID, cfg.SessionSecret)
		c.SetCookie("q_session", sc, 1800, "/", "", true, true)

		// 7. Clear q_admission (TOKEN-06 — consumed, must not be reused).
		c.SetCookie("q_admission", "", -1, "/", "", true, true)

		c.Next()
	}
}
