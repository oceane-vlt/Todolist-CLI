package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestIssueAndVerifyRoundTrip(t *testing.T) {
	secret := []byte("test-secret")
	issuer := NewIssuer(secret, time.Hour)
	verifier := NewHMACVerifier(secret)

	const userID = "user-123"
	const email = "user@example.com"
	token, expiresAt, err := issuer.Issue(userID, email)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if !expiresAt.After(time.Now()) {
		t.Fatalf("expiresAt = %v, want in the future", expiresAt)
	}

	got, err := verifier.Verify(token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if got.UserID != userID {
		t.Fatalf("Verify() userID = %q, want %q", got.UserID, userID)
	}
	if got.Email != email {
		t.Fatalf("Verify() email = %q, want %q", got.Email, email)
	}
}

func TestVerifyToleratesMissingEmailClaim(t *testing.T) {
	secret := []byte("test-secret")
	// A legacy sub-only token (no email claim) must still validate, with an
	// empty Email so the server treats provisioning as best-effort (skip).
	token, _, err := NewIssuer(secret, time.Hour).Issue("user-123", "")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	got, err := NewHMACVerifier(secret).Verify(token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if got.UserID != "user-123" {
		t.Fatalf("Verify() userID = %q, want %q", got.UserID, "user-123")
	}
	if got.Email != "" {
		t.Fatalf("Verify() email = %q, want empty", got.Email)
	}
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	secret := []byte("test-secret")
	issuer := NewIssuer(secret, -time.Minute) // already expired
	verifier := NewHMACVerifier(secret)

	token, _, err := issuer.Issue("user-123", "user@example.com")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	if _, err := verifier.Verify(token); err == nil {
		t.Fatal("Verify() succeeded for expired token, want error")
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	token, _, err := NewIssuer([]byte("secret-a"), time.Hour).Issue("user-123", "user@example.com")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if _, err := NewHMACVerifier([]byte("secret-b")).Verify(token); err == nil {
		t.Fatal("Verify() succeeded with wrong secret, want error")
	}
}

func TestVerifyRejectsNonHMACMethod(t *testing.T) {
	// A token signed with "none" must be rejected by the HS256-only verifier.
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{Subject: "user-123"})
	signed, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}
	if _, err := NewHMACVerifier([]byte("secret")).Verify(signed); err == nil {
		t.Fatal("Verify() accepted a 'none' token, want error")
	}
}

func TestVerifyRejectsMissingSubject(t *testing.T) {
	secret := []byte("test-secret")
	// Build a valid HS256 token with no subject claim.
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	signed, err := tok.SignedString(secret)
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}
	if _, err := NewHMACVerifier(secret).Verify(signed); err == nil {
		t.Fatal("Verify() accepted a token without subject, want error")
	}
}
