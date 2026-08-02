package token

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TestIssueAndVerify: sign with ADMISSION_SECRET, verify with same secret — no error, claims correct.
func TestIssueAndVerify(t *testing.T) {
	tok, err := IssueAdmission("ticket-abc", "evt-001", "secret-A")
	if err != nil {
		t.Fatalf("IssueAdmission error: %v", err)
	}
	claims, err := ValidateJWT(tok, "secret-A")
	if err != nil {
		t.Fatalf("ValidateJWT error: %v", err)
	}
	if claims.TicketID != "ticket-abc" {
		t.Errorf("expected TicketID=ticket-abc, got %s", claims.TicketID)
	}
	if claims.EventID != "evt-001" {
		t.Errorf("expected EventID=evt-001, got %s", claims.EventID)
	}
	if claims.Subject != "ticket-abc" {
		t.Errorf("expected Subject=ticket-abc, got %s", claims.Subject)
	}
}

// TestWrongSecretRejected: TOKEN-02 — token signed with secret-A must not verify with secret-B.
func TestWrongSecretRejected(t *testing.T) {
	tok, err := IssueAdmission("ticket-abc", "evt-001", "secret-A")
	if err != nil {
		t.Fatalf("IssueAdmission error: %v", err)
	}
	_, err = ValidateJWT(tok, "secret-B")
	if err == nil {
		t.Fatal("expected error when verifying with wrong secret, got nil")
	}
}

// TestExpiredToken: ValidateJWT must reject a token with exp in the past.
func TestExpiredToken(t *testing.T) {
	claims := &AdmissionClaims{
		EventID:  "evt-001",
		TicketID: "ticket-abc",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "ticket-abc",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Minute)),
			ID:        "test-jti",
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte("secret-A"))
	if err != nil {
		t.Fatalf("signing error: %v", err)
	}
	_, err = ValidateJWT(signed, "secret-A")
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

// TestJTIUniqueness: two IssueAdmission calls must produce different non-empty JTIs.
func TestJTIUniqueness(t *testing.T) {
	tok1, _ := IssueAdmission("ticket-1", "evt-001", "secret-A")
	tok2, _ := IssueAdmission("ticket-2", "evt-001", "secret-A")
	c1, err := ValidateJWT(tok1, "secret-A")
	if err != nil {
		t.Fatalf("validate tok1: %v", err)
	}
	c2, err := ValidateJWT(tok2, "secret-A")
	if err != nil {
		t.Fatalf("validate tok2: %v", err)
	}
	if c1.ID == "" || c2.ID == "" {
		t.Error("JTI must be non-empty")
	}
	if c1.ID == c2.ID {
		t.Errorf("JTIs must be unique, both got %s", c1.ID)
	}
}

// TestSessionSecretIsolation: an admission token signed with ADMISSION_SECRET must fail
// ValidateSession with SESSION_SECRET — the two-secret isolation guarantee.
func TestSessionSecretIsolation(t *testing.T) {
	tok, err := IssueAdmission("ticket-abc", "evt-001", "admission-secret")
	if err != nil {
		t.Fatalf("IssueAdmission error: %v", err)
	}
	_, err = ValidateSession(tok, "session-secret")
	if err == nil {
		t.Fatal("expected error: admission token must not validate as session token")
	}
}
