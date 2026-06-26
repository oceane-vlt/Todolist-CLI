package clientauth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/oceane-vlt/todolist/libs/auth"
)

// EnvJWTSigningKey is the shared secret for the local/home auth mode. It MUST
// match the server's JWT_SIGNING_KEY so the tokens the CLI mints validate
// server-side. This mode exists to de-risk the auth flow locally before wiring
// Supabase (docs/implementation-plan.md §3, §6).
const EnvJWTSigningKey = "JWT_SIGNING_KEY"

// devAccessTTL is the lifetime of locally-issued access tokens. Kept short so
// the refresh path is meaningful and testable.
const devAccessTTL = 15 * time.Minute

// Authenticator turns an email/password into Credentials and refreshes an
// expired access token. The CLI depends on this interface so the local/home
// dev implementation can be swapped for a Supabase HTTPS client later without
// touching the command layer.
type Authenticator interface {
	// Authenticate validates the email/password and returns fresh credentials.
	Authenticate(email, password, endpoint string) (*Credentials, error)
	// Refresh exchanges the refresh token for a new access token, updating the
	// credentials in place.
	Refresh(creds *Credentials) error
}

// DevAuthenticator is the local/home authentication mode. It derives a stable
// user_id from the email and mints HS256 access tokens with the shared signing
// key, so login/refresh work entirely offline against a local server sharing
// the same key.
//
// It is NOT a security boundary on its own (the password is not checked against
// any store): it is a development/test stand-in that exercises the real token
// plumbing. The production identity provider is Supabase Auth (see package doc).
type DevAuthenticator struct {
	issuer *auth.Issuer
}

// NewDevAuthenticatorFromEnv builds a DevAuthenticator from JWT_SIGNING_KEY.
// It returns (nil, false) when the key is absent, signalling that the home/dev
// mode is not configured (the caller should then direct the user to the
// Supabase flow or to set the key).
func NewDevAuthenticatorFromEnv() (*DevAuthenticator, bool) {
	secret := os.Getenv(EnvJWTSigningKey)
	if secret == "" {
		return nil, false
	}
	return &DevAuthenticator{issuer: auth.NewIssuer([]byte(secret), devAccessTTL)}, true
}

// Authenticate mints credentials for the given email. The password is required
// (non-empty) to mirror the real flow, but is not verified in this dev mode.
func (d *DevAuthenticator) Authenticate(email, password, endpoint string) (*Credentials, error) {
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if password == "" {
		return nil, fmt.Errorf("password is required")
	}

	userID := DevUserIDForEmail(email)
	// The email travels in the signed token (private "email" claim) so the server
	// can provision the user just-in-time (the FK target of lists.user_id).
	access, expiresAt, err := d.issuer.Issue(userID, email)
	if err != nil {
		return nil, err
	}

	return &Credentials{
		Email:        email,
		UserID:       userID,
		AccessToken:  access,
		RefreshToken: "dev-refresh-" + userID,
		ExpiresAt:    expiresAt,
		Endpoint:     endpoint,
	}, nil
}

// Refresh issues a new access token for the stored user, simulating a refresh
// against the provider. In dev mode the refresh token is opaque and the user_id
// is taken from the existing credentials.
func (d *DevAuthenticator) Refresh(creds *Credentials) error {
	if creds.RefreshToken == "" {
		return fmt.Errorf("no refresh token available; please log in again")
	}
	// Re-issue with the stored email so the refreshed token keeps the email claim
	// (needed for server-side provisioning after a refresh-and-replay).
	access, expiresAt, err := d.issuer.Issue(creds.UserID, creds.Email)
	if err != nil {
		return err
	}
	creds.AccessToken = access
	creds.ExpiresAt = expiresAt
	return nil
}

// DevUserIDForEmail derives a stable UUID (v5, URL namespace) from an email so
// the same email always maps to the same user_id across logins and devices —
// the multi-device property of docs/target-architecture.md §5.4, reproduced
// locally. In the Supabase target the user_id is the Supabase user UUID instead.
func DevUserIDForEmail(email string) string {
	sum := sha256.Sum256([]byte(email))
	// Use the email hash as the name within a fixed namespace for determinism.
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(hex.EncodeToString(sum[:]))).String()
}
