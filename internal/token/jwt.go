package token

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/adityabansal29/virtual-queue/internal/config"
)

// AdmissionClaims are the JWT claims for a q_admission cookie.
// JTI (claims.ID) is the SETNX key base; Subject mirrors TicketID for
// compatibility with DESIGN.md Section 8 (claims.Subject).
type AdmissionClaims struct {
	EventID  string `json:"eventId"`
	TicketID string `json:"ticketId"`
	jwt.RegisteredClaims
}

// IssueAdmission signs a new admission JWT (HMAC-SHA256) for the given ticketID
// and eventID using the supplied secret. A fresh UUID JTI guarantees uniqueness
// across tokens so SETNX key token:{jti} is never shared.
func IssueAdmission(ticketID, eventID, secret string) (string, error) {
	claims := &AdmissionClaims{
		EventID:  eventID,
		TicketID: ticketID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			Subject:   ticketID,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(config.AdmissionJWTTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// ValidateJWT parses and verifies a token signed with ADMISSION_SECRET.
// Returns claims only if the HMAC-SHA256 signature matches the given secret.
// Wrong secret, expired token, or malformed input all return a non-nil error.
func ValidateJWT(tokenString, secret string) (*AdmissionClaims, error) {
	tok, err := jwt.ParseWithClaims(
		tokenString,
		&AdmissionClaims{},
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
	claims, ok := tok.Claims.(*AdmissionClaims)
	if !ok || !tok.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}
