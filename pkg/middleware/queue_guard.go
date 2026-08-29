package middleware

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/adityabansal29/virtual-queue/internal/config"
	"github.com/adityabansal29/virtual-queue/internal/token"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// Config holds the parameters for the QueueGuard middleware.
// RDB must point to the redis-origin client, NOT redis-queue.
type Config struct {
	AdmissionSecret string
	SessionSecret   string
	QueueJoinURL    string // GET /queue/join endpoint — linked from the error page
	EventID         string // event this origin belongs to
	Secure          bool   // true in production (HTTPS), false for local HTTP dev
	RDB             *redis.Client
}

// QueueGuard enforces the two-cookie token model (DESIGN.md §8).
//
// Fast path:   valid q_session → pass through.
// Admission:   valid q_admission → SETNX (TOKEN-04) → issue q_session → pass through.
// No cookie:   return 401 error page with a manual "Join the queue" link.
//              No auto-redirect — avoids origin→EW dependency and infinite-loop risk.
func QueueGuard(cfg Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Session fast-path.
		if sc, err := c.Cookie("q_session"); err == nil {
			if claims, err := token.ValidateSession(sc, cfg.SessionSecret); err == nil {
				c.Set("session", claims)
				c.Next()
				return
			}
		}

		// 2. Admission token required.
		ac, err := c.Cookie("q_admission")
		if err != nil {
			c.Data(http.StatusUnauthorized, "text/html; charset=utf-8", []byte(queueErrorPage(cfg, c.Request.URL.String())))
			c.Abort()
			return
		}

		// 3. Verify JWT.
		claims, err := token.ValidateJWT(ac, cfg.AdmissionSecret)
		if err != nil {
			c.Data(http.StatusUnauthorized, "text/html; charset=utf-8", []byte(queueErrorPage(cfg, c.Request.URL.String())))
			c.Abort()
			return
		}

		// 4. SETNX — one-time enforcement (TOKEN-04).
		set, err := cfg.RDB.SetNX(c.Request.Context(), "token:"+claims.ID, "used", config.AdmissionUsedTTL).Result()
		if err != nil || !set {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		// 5. Issue q_session, clear q_admission, store claims for this request.
		sc, _ := token.IssueSession(claims.Subject, claims.EventID, cfg.SessionSecret)
		c.SetCookie("q_session", sc, config.QSessionCookieMaxAge, "/", "", cfg.Secure, true)
		c.SetCookie("q_admission", "", -1, "/", "", cfg.Secure, true)
		// Cookie is set on the response; handler reads from gin context on this request.
		if sessionClaims, err := token.ValidateSession(sc, cfg.SessionSecret); err == nil {
			c.Set("session", sessionClaims)
		}

		c.Next()
	}
}

func queueErrorPage(cfg Config, requestURL string) string {
	joinLink := fmt.Sprintf("%s?eventId=%s&target=%s",
		cfg.QueueJoinURL, url.QueryEscape(cfg.EventID), url.QueryEscape(requestURL))
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Queue Required</title>
</head>
<body style="max-width:480px;margin:64px auto;font-family:system-ui,-apple-system,sans-serif">
<h1 style="font-size:20px;font-weight:600">You need a queue ticket to access this page.</h1>
<p style="color:#6b7280">Join the waiting room to get in line. You will be redirected automatically when admitted.</p>
<a href="%s" style="display:inline-block;margin-top:16px;padding:10px 20px;background:#2563eb;color:#fff;border-radius:4px;text-decoration:none">Join the queue</a>
</body>
</html>`, joinLink)
}
