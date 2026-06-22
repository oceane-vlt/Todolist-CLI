package clientauth

import (
	"errors"
	"testing"
)

func TestNewAuthenticatorFromEnvSelectsSupabase(t *testing.T) {
	t.Setenv(EnvSupabaseURL, "https://ref.supabase.co")
	t.Setenv(EnvSupabaseAnonKey, "anon-key")
	// A leftover signing key must NOT win: Supabase has precedence.
	t.Setenv(EnvJWTSigningKey, "leftover-secret")

	a, err := NewAuthenticatorFromEnv()
	if err != nil {
		t.Fatalf("NewAuthenticatorFromEnv() error = %v", err)
	}
	if _, ok := a.(*SupabaseAuthenticator); !ok {
		t.Fatalf("got %T, want *SupabaseAuthenticator when SUPABASE_URL+ANON_KEY set", a)
	}
	// Supabase has a distinct signup endpoint, so it must satisfy SignUpAuthenticator.
	if _, ok := a.(SignUpAuthenticator); !ok {
		t.Error("SupabaseAuthenticator should implement SignUpAuthenticator")
	}
}

func TestNewAuthenticatorFromEnvSelectsDev(t *testing.T) {
	t.Setenv(EnvSupabaseURL, "")
	t.Setenv(EnvSupabaseAnonKey, "")
	t.Setenv(EnvJWTSigningKey, "shared-secret")

	a, err := NewAuthenticatorFromEnv()
	if err != nil {
		t.Fatalf("NewAuthenticatorFromEnv() error = %v", err)
	}
	if _, ok := a.(*DevAuthenticator); !ok {
		t.Fatalf("got %T, want *DevAuthenticator when only JWT_SIGNING_KEY set", a)
	}
	// Dev mode has no distinct signup: signup==login, so it must NOT implement
	// SignUpAuthenticator (the command layer falls back to Authenticate).
	if _, ok := a.(SignUpAuthenticator); ok {
		t.Error("DevAuthenticator should NOT implement SignUpAuthenticator")
	}
}

func TestNewAuthenticatorFromEnvPartialSupabaseFallsThrough(t *testing.T) {
	// Only one of the two Supabase vars set -> Supabase not configured; fall
	// through to the dev mode.
	t.Setenv(EnvSupabaseURL, "https://ref.supabase.co")
	t.Setenv(EnvSupabaseAnonKey, "")
	t.Setenv(EnvJWTSigningKey, "shared-secret")

	a, err := NewAuthenticatorFromEnv()
	if err != nil {
		t.Fatalf("NewAuthenticatorFromEnv() error = %v", err)
	}
	if _, ok := a.(*DevAuthenticator); !ok {
		t.Fatalf("got %T, want *DevAuthenticator when Supabase is only partially configured", a)
	}
}

func TestNewAuthenticatorFromEnvNoneConfigured(t *testing.T) {
	t.Setenv(EnvSupabaseURL, "")
	t.Setenv(EnvSupabaseAnonKey, "")
	t.Setenv(EnvJWTSigningKey, "")

	_, err := NewAuthenticatorFromEnv()
	if !errors.Is(err, ErrNoAuthMode) {
		t.Fatalf("NewAuthenticatorFromEnv() error = %v, want ErrNoAuthMode", err)
	}
}
