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
	cfg := config.LoadStubOrigin()

	slog.Info("stub origin starting",
		"port", "8081",
		"redis", cfg.RedisAddr,
		"admission_secret", "set",
		"session_secret", "set",
	)

	originRedis := store.NewQueueRedis(cfg.RedisAddr)

	mwCfg := middleware.Config{
		AdmissionSecret: cfg.AdmissionSecret,
		SessionSecret:   cfg.SessionSecret,
		QueueJoinURL:    cfg.QueueJoinURL,
		EventID:         cfg.EventID,
		Secure:          cfg.Secure,
		RDB:             originRedis,
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	r.GET("/", middleware.QueueGuard(mwCfg), func(c *gin.Context) {
		val, _ := c.Get("session")
		claims, ok := val.(*token.SessionClaims)
		if !ok || claims == nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, checkoutPage(claims.EventID, claims.Subject))
	})

	if err := r.Run(":8081"); err != nil {
		slog.Error("stub origin failed", "error", err)
	}
}

// checkoutPage renders the seat selection page (D-11, UI-08).
// T-02-10: eventID embedded via fmt.Sprintf("%q") — prevents XSS from crafted eventId values.
func checkoutPage(eventID, ticketID string) string {
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
    btn.disabled = true;
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
