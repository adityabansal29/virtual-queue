package main

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/adityabansal29/virtual-queue/internal/config"
	"github.com/adityabansal29/virtual-queue/internal/store"
	"github.com/adityabansal29/virtual-queue/internal/token"
	"github.com/adityabansal29/virtual-queue/pkg/middleware"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	slog.Info("stub origin starting",
		"port", "8081",
		"redis", cfg.RedisAddr,
		// T-01-05: never log secret values.
		"admission_secret", "set",
		"session_secret", "set",
	)

	// redis-origin client — REDIS_ADDR is set to redis-origin:6379 in Docker Compose.
	// D-02/D-03: stub origin uses redis-origin, NOT redis-queue.
	originRedis := store.NewQueueRedis(cfg.RedisAddr)

	mwCfg := middleware.Config{
		AdmissionSecret: cfg.AdmissionSecret,
		SessionSecret:   cfg.SessionSecret,
		QueueURL:        "http://localhost:8080",
		RDB:             originRedis,
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// D-10/D-11/D-12: validate q_session, render seat grid, or session-expired error page.
	r.GET("/", middleware.QueueGuard(mwCfg), func(c *gin.Context) {
		cookie, err := c.Cookie("q_session")
		if err != nil {
			c.Header("Content-Type", "text/html")
			c.String(http.StatusUnauthorized, errorPage())
			return
		}
		claims, err := token.ValidateSession(cookie, cfg.SessionSecret)
		if err != nil {
			c.Header("Content-Type", "text/html")
			c.String(http.StatusUnauthorized, errorPage())
			return
		}
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, checkoutPage(claims.EventID, claims.Subject))
	})

	if err := r.Run(":8081"); err != nil {
		slog.Error("stub origin failed", "error", err)
	}
}

// errorPage renders the session-expired error page (D-12, UI-08).
func errorPage() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Virtual Queue — Session Expired</title>
</head>
<body style="max-width:560px;margin:48px auto;font-family:system-ui,-apple-system,sans-serif">
<h1 style="font-size:20px;font-weight:600">Your session has expired.</h1>
<p style="font-size:16px;color:#6b7280">Return to the queue to rejoin.</p>
<a href="http://localhost:8082/queue/" style="color:#2563eb">Return to queue</a>
</body>
</html>`
}

// checkoutPage renders the seat selection page (D-11, UI-08).
// T-02-10: eventID is embedded via fmt.Sprintf("%q") which produces a Go-quoted
// (JSON-compatible) string literal — prevents XSS from crafted eventId values.
func checkoutPage(eventID, ticketID string) string {
	// T-02-10: JSON-safe injection of eventID into inline script.
	safeEventID := fmt.Sprintf("%q", eventID)
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Ticket Checkout</title>
<style>
body{max-width:560px;margin:48px auto;font-family:system-ui,-apple-system,sans-serif}
p{overflow-wrap:break-word}
.seat-grid{display:grid;grid-template-columns:repeat(3,48px);gap:8px;margin:24px 0}
.seat{width:48px;height:48px;border:1px solid #e5e7eb;border-radius:4px;cursor:pointer;font-size:14px;background:#fff}
.seat.selected{background:#2563eb;color:#fff;border-color:#2563eb}
#complete-btn{width:100%%;padding:16px;background:#2563eb;color:#fff;border:none;border-radius:4px;font-size:16px;cursor:pointer;margin-top:16px}
#complete-btn:disabled{opacity:0.5;cursor:not-allowed}
#success{display:none;padding:16px;background:#f3f4f6;border-radius:4px;margin-top:16px}
#exit-error{color:#dc2626;font-size:14px;margin-top:8px;display:none}
</style>
</head>
<body>
<p>Event: %s</p>
<p>Ticket: %s</p>
<div class="seat-grid">%s</div>
<button id="complete-btn">Complete Purchase</button>
<div id="success">Thank you &mdash; your ticket is confirmed.</div>
<p id="exit-error">Could not free slot. Please close this tab.</p>
<script>
// T-02-10: eventId injected as a Go-quoted JSON string literal.
var __eventId = %s;
(function(){
  var seats = document.querySelectorAll('.seat');
  seats.forEach(function(s){
    s.addEventListener('click', function(){
      seats.forEach(function(x){ x.classList.remove('selected'); });
      s.classList.add('selected');
    });
  });
  document.getElementById('complete-btn').addEventListener('click', function(){
    var btn = document.getElementById('complete-btn');
    btn.disabled = true; // prevent double-submit (UI-08)
    fetch('http://localhost:8080/queue/exit', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({eventId: __eventId})
    }).then(function(res){
      if (!res.ok) throw new Error('non-200');
      btn.style.display = 'none';
      document.getElementById('success').style.display = 'block';
    }).catch(function(){
      btn.disabled = false;
      document.getElementById('exit-error').style.display = 'block';
    });
  });
})();
</script>
</body>
</html>`, eventID, ticketID, seatButtons(), safeEventID)
}

// seatButtons returns the 12 seat button HTML elements (3x4 grid).
// Seat 1 is pre-selected per UI-SPEC.
func seatButtons() string {
	out := ""
	for i := 1; i <= 12; i++ {
		cls := "seat"
		if i == 1 {
			cls = "seat selected"
		}
		out += fmt.Sprintf(`<button class=%q>Seat %d</button>`, cls, i)
	}
	return out
}
