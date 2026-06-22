package server

import (
	"context"
	"testing"
	"time"

	"github.com/oceane-vlt/todolist/libs/auth"
	"github.com/oceane-vlt/todolist/libs/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const testSecret = "interceptor-test-secret"

// fakeProvisioner records EnsureUser calls so tests can assert provisioning
// happened with the expected user_id/email derived from the signed token.
type fakeProvisioner struct {
	calls    int
	lastID   string
	lastMail string
	err      error
}

func (f *fakeProvisioner) EnsureUser(_ context.Context, userID, email string) error {
	f.calls++
	f.lastID = userID
	f.lastMail = email
	return f.err
}

// invoke runs the AuthInterceptor with the given incoming metadata and optional
// provisioner, returning the user_id the handler observed (empty if the handler
// was not reached).
func invoke(t *testing.T, md metadata.MD, provisioner UserProvisioner) (observedUserID string, err error) {
	t.Helper()
	interceptor := AuthInterceptor(auth.NewHMACVerifier([]byte(testSecret)), provisioner)

	ctx := context.Background()
	if md != nil {
		ctx = metadata.NewIncomingContext(ctx, md)
	}

	handler := func(ctx context.Context, _ any) (any, error) {
		if id, ok := storage.UserIDFromContext(ctx); ok {
			observedUserID = id
		}
		return nil, nil
	}
	_, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
	return observedUserID, err
}

func validToken(t *testing.T, userID, email string) string {
	t.Helper()
	token, _, err := auth.NewIssuer([]byte(testSecret), time.Hour).Issue(userID, email)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	return token
}

func TestAuthInterceptorAcceptsValidTokenAndInjectsUserID(t *testing.T) {
	md := metadata.Pairs("authorization", "Bearer "+validToken(t, "user-42", "u42@example.com"))
	got, err := invoke(t, md, nil)
	if err != nil {
		t.Fatalf("interceptor error = %v, want nil", err)
	}
	if got != "user-42" {
		t.Fatalf("injected user_id = %q, want %q", got, "user-42")
	}
}

// TestAuthInterceptorProvisionsUserFromTokenEmail is the core proof that
// just-in-time provisioning fires with the right identity derived from the
// SIGNED token (not from any client field): the interceptor must call
// EnsureUser(userID, email) before the handler runs.
func TestAuthInterceptorProvisionsUserFromTokenEmail(t *testing.T) {
	prov := &fakeProvisioner{}
	md := metadata.Pairs("authorization", "Bearer "+validToken(t, "user-42", "u42@example.com"))
	got, err := invoke(t, md, prov)
	if err != nil {
		t.Fatalf("interceptor error = %v, want nil", err)
	}
	if got != "user-42" {
		t.Fatalf("injected user_id = %q, want %q", got, "user-42")
	}
	if prov.calls != 1 {
		t.Fatalf("EnsureUser called %d times, want 1", prov.calls)
	}
	if prov.lastID != "user-42" || prov.lastMail != "u42@example.com" {
		t.Fatalf("EnsureUser(%q,%q), want (%q,%q)", prov.lastID, prov.lastMail, "user-42", "u42@example.com")
	}
}

// TestAuthInterceptorSkipsProvisioningWhenNoEmail ensures legacy sub-only tokens
// do not trigger provisioning (best-effort), avoiding a spurious EnsureUser call
// with an empty email.
func TestAuthInterceptorSkipsProvisioningWhenNoEmail(t *testing.T) {
	prov := &fakeProvisioner{}
	md := metadata.Pairs("authorization", "Bearer "+validToken(t, "user-42", ""))
	if _, err := invoke(t, md, prov); err != nil {
		t.Fatalf("interceptor error = %v, want nil", err)
	}
	if prov.calls != 0 {
		t.Fatalf("EnsureUser called %d times, want 0 for email-less token", prov.calls)
	}
}

// TestAuthInterceptorProvisioningFailureSurfaces verifies a provisioning error
// is surfaced as a gRPC error and the handler is not reached.
func TestAuthInterceptorProvisioningFailureSurfaces(t *testing.T) {
	prov := &fakeProvisioner{err: errProvision}
	md := metadata.Pairs("authorization", "Bearer "+validToken(t, "user-42", "u42@example.com"))
	got, err := invoke(t, md, prov)
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal", status.Code(err))
	}
	if got != "" {
		t.Fatalf("handler reached (user_id=%q), want not reached", got)
	}
}

var errProvision = status.Error(codes.Unknown, "boom")

func TestAuthInterceptorRejectsMissingMetadata(t *testing.T) {
	_, err := invoke(t, nil, nil)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("code = %v, want Unauthenticated", status.Code(err))
	}
}

func TestAuthInterceptorRejectsMissingHeader(t *testing.T) {
	_, err := invoke(t, metadata.Pairs("other", "x"), nil)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("code = %v, want Unauthenticated", status.Code(err))
	}
}

func TestAuthInterceptorRejectsNonBearer(t *testing.T) {
	_, err := invoke(t, metadata.Pairs("authorization", "Basic abc"), nil)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("code = %v, want Unauthenticated", status.Code(err))
	}
}

func TestAuthInterceptorRejectsInvalidToken(t *testing.T) {
	_, err := invoke(t, metadata.Pairs("authorization", "Bearer not-a-jwt"), nil)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("code = %v, want Unauthenticated", status.Code(err))
	}
}

func TestAuthInterceptorRejectsExpiredToken(t *testing.T) {
	expired, _, err := auth.NewIssuer([]byte(testSecret), -time.Minute).Issue("user-42", "u42@example.com")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	_, err = invoke(t, metadata.Pairs("authorization", "Bearer "+expired), nil)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("code = %v, want Unauthenticated", status.Code(err))
	}
}

func TestAuthInterceptorFromEnvSelection(t *testing.T) {
	t.Run("signing key enables auth", func(t *testing.T) {
		t.Setenv(EnvJWTSigningKey, "k")
		t.Setenv(EnvSupabaseJWTSecret, "")
		if _, enabled := AuthInterceptorFromEnv("dev", nil); !enabled {
			t.Fatal("authEnabled = false, want true when JWT_SIGNING_KEY set")
		}
	})
	t.Run("supabase secret enables auth", func(t *testing.T) {
		t.Setenv(EnvJWTSigningKey, "")
		t.Setenv(EnvSupabaseJWTSecret, "s")
		if _, enabled := AuthInterceptorFromEnv("dev", nil); !enabled {
			t.Fatal("authEnabled = false, want true when SUPABASE_JWT_SECRET set")
		}
	})
	t.Run("neither falls back to dev interceptor", func(t *testing.T) {
		t.Setenv(EnvJWTSigningKey, "")
		t.Setenv(EnvSupabaseJWTSecret, "")
		if _, enabled := AuthInterceptorFromEnv("dev", nil); enabled {
			t.Fatal("authEnabled = true, want false when no auth env set")
		}
	})
	t.Run("SUPABASE_URL enables JWKS mode", func(t *testing.T) {
		t.Setenv(EnvJWTSigningKey, "")
		t.Setenv(EnvSupabaseJWTSecret, "")
		t.Setenv(EnvSupabaseJWKSURL, "")
		t.Setenv(EnvSupabaseURL, "https://example.supabase.co")
		if _, enabled := AuthInterceptorFromEnv("dev", nil); !enabled {
			t.Fatal("authEnabled = false, want true when SUPABASE_URL set (JWKS mode)")
		}
	})
	t.Run("SUPABASE_JWKS_URL enables JWKS mode", func(t *testing.T) {
		t.Setenv(EnvJWTSigningKey, "")
		t.Setenv(EnvSupabaseJWTSecret, "")
		t.Setenv(EnvSupabaseURL, "")
		t.Setenv(EnvSupabaseJWKSURL, "https://example.supabase.co/auth/v1/.well-known/jwks.json")
		if _, enabled := AuthInterceptorFromEnv("dev", nil); !enabled {
			t.Fatal("authEnabled = false, want true when SUPABASE_JWKS_URL set (JWKS mode)")
		}
	})
	t.Run("JWT_SIGNING_KEY wins over SUPABASE_URL", func(t *testing.T) {
		// Home/dev secret must keep top priority even when SUPABASE_URL is set:
		// a token signed with the home HS256 secret is accepted, proving the
		// HMACVerifier (not the JWKSVerifier) was wired.
		t.Setenv(EnvJWTSigningKey, testSecret)
		t.Setenv(EnvSupabaseURL, "https://example.supabase.co")
		t.Setenv(EnvSupabaseJWTSecret, "")
		t.Setenv(EnvSupabaseJWKSURL, "")
		interceptor, enabled := AuthInterceptorFromEnv("dev", nil)
		if !enabled {
			t.Fatal("authEnabled = false, want true when JWT_SIGNING_KEY set")
		}
		md := metadata.Pairs("authorization", "Bearer "+validToken(t, "user-7", "u7@example.com"))
		got, err := invokeWith(t, interceptor, md)
		if err != nil {
			t.Fatalf("home HS256 token rejected: %v (JWKSVerifier wired instead of HMACVerifier?)", err)
		}
		if got != "user-7" {
			t.Fatalf("injected user_id = %q, want %q", got, "user-7")
		}
	})
	t.Run("JWKS chosen over legacy HS256 secret", func(t *testing.T) {
		// Both SUPABASE_URL and SUPABASE_JWT_SECRET set: JWKS must win (Option B).
		// Proof: a token signed with the legacy HS256 secret is REJECTED, because
		// the JWKSVerifier only accepts ES256/RS256, not HS256.
		t.Setenv(EnvJWTSigningKey, "")
		t.Setenv(EnvSupabaseURL, "https://example.supabase.co")
		t.Setenv(EnvSupabaseJWTSecret, testSecret)
		t.Setenv(EnvSupabaseJWKSURL, "")
		interceptor, enabled := AuthInterceptorFromEnv("dev", nil)
		if !enabled {
			t.Fatal("authEnabled = false, want true")
		}
		legacyHS256 := metadata.Pairs("authorization", "Bearer "+validToken(t, "user-9", "u9@example.com"))
		if _, err := invokeWith(t, interceptor, legacyHS256); status.Code(err) != codes.Unauthenticated {
			t.Fatalf("legacy HS256 token: code = %v, want Unauthenticated (HMACVerifier wired instead of JWKSVerifier?)", status.Code(err))
		}
	})
}

// invokeWith runs an already-selected interceptor with the given incoming
// metadata, returning the user_id the handler observed (empty if not reached).
func invokeWith(t *testing.T, interceptor grpc.UnaryServerInterceptor, md metadata.MD) (observedUserID string, err error) {
	t.Helper()
	ctx := context.Background()
	if md != nil {
		ctx = metadata.NewIncomingContext(ctx, md)
	}
	handler := func(ctx context.Context, _ any) (any, error) {
		if id, ok := storage.UserIDFromContext(ctx); ok {
			observedUserID = id
		}
		return nil, nil
	}
	_, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
	return observedUserID, err
}
