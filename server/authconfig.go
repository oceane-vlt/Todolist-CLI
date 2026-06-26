package server

import (
	"os"
	"strings"

	"github.com/oceane-vlt/todolist/libs/auth"
	"google.golang.org/grpc"
)

// Environment variables controlling server-side authentication (Phase 3).
const (
	// EnvJWTSigningKey is the shared secret for the local/home auth mode. When
	// set, the server validates HS256 tokens signed with this key. This is the
	// "locally signed JWT" path the plan de-risks before Supabase.
	EnvJWTSigningKey = "JWT_SIGNING_KEY"

	// EnvSupabaseJWTSecret is Supabase's legacy project JWT secret (HS256). When
	// set (and neither JWT_SIGNING_KEY nor the JWKS mode applies), the server
	// validates Supabase-issued access tokens against this shared secret. This is
	// the "Option A" legacy path, kept as a fallback now that Supabase projects
	// default to asymmetric (ES256) signing keys.
	EnvSupabaseJWTSecret = "SUPABASE_JWT_SECRET"

	// EnvSupabaseURL is the bare Supabase project URL (e.g.
	// https://<ref>.supabase.co), shared with the CLI. When set (and
	// JWT_SIGNING_KEY is not), the server enables JWKS/asymmetric verification by
	// deriving the JWKS endpoint as {SUPABASE_URL}/auth/v1/.well-known/jwks.json.
	EnvSupabaseURL = "SUPABASE_URL"

	// EnvSupabaseJWKSURL overrides the derived JWKS endpoint with a full URL. It
	// takes precedence over EnvSupabaseURL for selecting the JWKS mode and is
	// handy for tests or non-default deployments.
	EnvSupabaseJWKSURL = "SUPABASE_JWKS_URL"
)

// supabaseJWKSPath is appended to a bare SUPABASE_URL to derive the JWKS
// endpoint. It mirrors the "/auth/v1" prefix the CLI uses for GoTrue.
const supabaseJWKSPath = "/auth/v1/.well-known/jwks.json"

// jwksURLFromEnv resolves the JWKS endpoint from the environment, preferring an
// explicit SUPABASE_JWKS_URL over one derived from SUPABASE_URL. It returns ""
// when neither is set (JWKS mode disabled).
func jwksURLFromEnv() string {
	if u := os.Getenv(EnvSupabaseJWKSURL); u != "" {
		return u
	}
	if base := os.Getenv(EnvSupabaseURL); base != "" {
		return strings.TrimRight(base, "/") + supabaseJWKSPath
	}
	return ""
}

// AuthInterceptorFromEnv selects the unary interceptor to use based on the
// environment, returning the interceptor and whether real authentication is
// enabled.
//
// Precedence (most specific to fallback):
//
//   - JWT_SIGNING_KEY set                  -> AuthInterceptor (local/home HS256
//     mode). Kept first so a developer can always force the home secret.
//   - SUPABASE_JWKS_URL or SUPABASE_URL set -> AuthInterceptor with a
//     JWKSVerifier (Supabase asymmetric ES256/RS256 mode, the production target
//     now that Supabase projects sign with an active ES256 key).
//   - SUPABASE_JWT_SECRET set              -> AuthInterceptor (Supabase legacy
//     HS256 "Option A" fallback).
//   - none set                             -> DevUserIDInterceptor(devUserID)
//     (Phase 2 fallback), so the default local run keeps working with no auth
//     config.
//
// JWKS is placed ahead of the legacy HS256 secret because the project's active
// signing key is ES256; an operator who deliberately wants HS256 sets only
// SUPABASE_JWT_SECRET (without SUPABASE_URL/SUPABASE_JWKS_URL). The JWKS URL is
// not fetched here: NewJWKSVerifier only validates that the URL is non-empty
// (the key set is fetched lazily on the first Verify), so server startup never
// blocks on JWKS reachability. The err check below is a defensive guard; since
// jwksURLFromEnv returned a non-empty URL it does not fail in practice.
//
// provisioner is passed through to AuthInterceptor so the authenticated user is
// provisioned just-in-time (it should be the *storage.PgStore when the backend
// is Postgres, otherwise nil). It is only used when authentication is enabled.
//
// Keeping the unauthenticated dev fallback as the default is what preserves the
// "the local default must keep working" guarantee between phases.
func AuthInterceptorFromEnv(devUserID string, provisioner UserProvisioner) (interceptor grpc.UnaryServerInterceptor, authEnabled bool) {
	if secret := os.Getenv(EnvJWTSigningKey); secret != "" {
		return AuthInterceptor(auth.NewHMACVerifier([]byte(secret)), provisioner), true
	}
	if jwksURL := jwksURLFromEnv(); jwksURL != "" {
		if verifier, err := auth.NewJWKSVerifier(jwksURL); err == nil {
			return AuthInterceptor(verifier, provisioner), true
		}
		// Defensive only: NewJWKSVerifier just rejects an empty URL, and jwksURL
		// is non-empty here. If that contract ever changes, fall through to the
		// legacy HS256 or dev fallback instead of crashing.
	}
	if secret := os.Getenv(EnvSupabaseJWTSecret); secret != "" {
		return AuthInterceptor(auth.NewHMACVerifier([]byte(secret)), provisioner), true
	}
	return DevUserIDInterceptor(devUserID), false
}
