package token

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/adityabansal29/virtual-queue/internal/config"
)

// SessionClaims are the JWT claims for a q_session cookie.
// Signed with SESSION_SECRET — a different key from ADMISSION_SECRET.
// A session token cannot be verified as an admission token, and vice versa.
type SessionClaims struct {
	EventID string `json:"eventId"`
	jwt.RegisteredClaims
}

// IssueSession signs a new session JWT (HMAC-SHA256) for the given ticketID and
// eventID using the supplied SESSION_SECRET. Same signing pattern as IssueAdmission
// but with SessionClaims so the two token types cannot cross-verify (TOKEN-02).
func IssueSession(ticketID, eventID, secret string) (string, error) {
	claims := &SessionClaims{
		EventID: eventID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			Subject:   ticketID,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(config.SessionJWTTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// ValidateSession parses and verifies a session token signed with SESSION_SECRET.
// Returns SessionClaims if valid, error otherwise.
func ValidateSession(tokenString, secret string) (*SessionClaims, error) {
	tok, err := jwt.ParseWithClaims(
		tokenString,
		&SessionClaims{},
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(secret), nil
		},
	)
	if err != nil {
		return nil, err
	}
	claims, ok := tok.Claims.(*SessionClaims)
	if !ok || !tok.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}
