// Package auth provides JWT verification (server side) and a local development
// token issuer (Phase 3 of docs/implementation-plan.md).
//
// The plan mandates de-risking authentication "with a locally signed JWT first,
// then Supabase". This package therefore supports two verification modes behind
// a single Verifier interface:
//
//   - Local/home mode (HS256): tokens are signed and verified with a shared
//     secret (JWT_SIGNING_KEY). The CLI can mint these tokens itself in dev mode
//     (Issuer below), so the whole interceptor/refresh flow is exercised end to
//     end without any external account. This is the default for local runs and
//     for the test suite.
//
//   - Supabase mode (HS256 with SUPABASE_JWT_SECRET): Supabase signs access
//     tokens with a project secret; the server validates the signature and reads
//     the subject claim. An RS256/JWKS path (SUPABASE_JWKS_URL) is documented as
//     the production hardening step (see docs/target-architecture.md §5) but is
//     intentionally left to a follow-up so this phase has no network dependency.
//
// In all modes the authenticated user identity is the JWT "sub" (subject) claim,
// which the gRPC interceptor injects into the context via storage.WithUserID.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrNoSubject is returned when a token is otherwise valid but carries no
// subject claim to identify the user.
var ErrNoSubject = errors.New("auth: token has no subject (sub) claim")

// Identity is the authenticated caller derived from a validated token. UserID
// is the JWT "sub" claim (used to scope storage per user). Email is the private
// "email" claim, which the server uses to provision the user just-in-time in the
// users table (the FK target of lists.user_id). Email is empty for legacy
// sub-only tokens; callers must treat provisioning as best-effort in that case.
type Identity struct {
	UserID string
	Email  string
}

// Verifier validates an access token and returns the authenticated identity
// (the JWT subject plus, when present, the email claim). Implementations must
// reject expired or malformed tokens.
type Verifier interface {
	// Verify parses and validates the raw access token. On success it returns
	// the authenticated Identity (the "sub" claim, plus "email" if present).
	Verify(token string) (Identity, error)
}

// emailClaim is the private JWT claim carrying the user's email. It is embedded
// in the signed token so the server can trust it (it never comes from a
// client-supplied field, per docs/target-architecture.md §4.1).
const emailClaim = "email"

// tokenClaims are the registered claims plus the private email claim. Keeping
// email in the signed payload is what makes just-in-time user provisioning
// non-spoofable.
type tokenClaims struct {
	jwt.RegisteredClaims
	Email string `json:"email,omitempty"`
}

// HMACVerifier validates HS256 JWTs signed with a shared secret. It backs both
// the local/home dev mode (JWT_SIGNING_KEY) and Supabase's symmetric secret
// mode (SUPABASE_JWT_SECRET): the two differ only in where the secret comes
// from, not in how the token is validated.
type HMACVerifier struct {
	secret []byte
}

// NewHMACVerifier returns a Verifier for HS256 tokens signed with secret.
func NewHMACVerifier(secret []byte) *HMACVerifier {
	return &HMACVerifier{secret: secret}
}

// Verify implements Verifier. It enforces the HS256 signing method, signature
// validity and expiration, then returns the subject claim and the optional
// email claim as an Identity.
func (v *HMACVerifier) Verify(token string) (Identity, error) {
	var claims tokenClaims
	_, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("auth: unexpected signing method %q", t.Header["alg"])
		}
		return v.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return Identity{}, fmt.Errorf("auth: invalid token: %w", err)
	}

	if claims.Subject == "" {
		return Identity{}, ErrNoSubject
	}
	return Identity{UserID: claims.Subject, Email: claims.Email}, nil
}

// Issuer mints HS256 access tokens locally. It exists only for the local/home
// (dev) authentication mode and the tests: in the Supabase target, tokens are
// issued by Supabase Auth over HTTPS, not by this CLI/server. Keeping the issuer
// behind the same secret as the HMACVerifier is what lets the full login →
// metadata → interceptor → refresh flow be validated locally first.
type Issuer struct {
	secret []byte
	ttl    time.Duration
}

// NewIssuer returns an Issuer that signs tokens valid for ttl with secret.
func NewIssuer(secret []byte, ttl time.Duration) *Issuer {
	return &Issuer{secret: secret, ttl: ttl}
}

// Issue returns a signed HS256 access token for the given user id and email,
// plus its expiry time. The subject claim carries the user id and the private
// "email" claim carries the email so the verifier (and the gRPC interceptor)
// can recover the identity and provision the user just-in-time. An empty email
// is allowed (the claim is simply omitted).
func (i *Issuer) Issue(userID, email string) (token string, expiresAt time.Time, err error) {
	now := time.Now()
	expiresAt = now.Add(i.ttl)
	claims := tokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		Email: email,
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("auth: signing token: %w", err)
	}
	return signed, expiresAt, nil
}
