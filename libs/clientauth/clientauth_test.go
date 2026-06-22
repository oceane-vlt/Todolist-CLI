package clientauth

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/oceane-vlt/todolist/libs/auth"
)

func TestSaveEnforces0600AndRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), CredentialsFileName)
	creds := &Credentials{
		Email:        "me@example.com",
		UserID:       "user-1",
		AccessToken:  "access",
		RefreshToken: "refresh",
		ExpiresAt:    time.Now().Add(time.Hour).Round(time.Second),
		Endpoint:     "127.0.0.1:50051",
	}

	if err := Save(path, creds); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat() error = %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0600 {
			t.Fatalf("credentials file mode = %v, want 0600", perm)
		}
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Email != creds.Email || got.UserID != creds.UserID || got.AccessToken != creds.AccessToken {
		t.Fatalf("Load() = %+v, want %+v", got, creds)
	}
}

func TestSaveReappliesPermsOnExistingLooseFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits not enforced on Windows")
	}
	path := filepath.Join(t.TempDir(), CredentialsFileName)
	if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
		t.Fatalf("seed write error = %v", err)
	}
	if err := Save(path, &Credentials{UserID: "u"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	info, _ := os.Stat(path)
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("mode = %v, want 0600 after Save over a 0644 file", perm)
	}
}

func TestLoadMissingReturnsNotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.json")
	_, err := Load(path)
	if !os.IsNotExist(err) {
		t.Fatalf("Load() error = %v, want os.IsNotExist", err)
	}
}

func TestDeleteIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), CredentialsFileName)
	if err := Save(path, &Credentials{UserID: "u"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := Delete(path); err != nil {
		t.Fatalf("Delete() first call error = %v", err)
	}
	if err := Delete(path); err != nil {
		t.Fatalf("Delete() on missing file error = %v, want nil", err)
	}
}

func TestExpired(t *testing.T) {
	cases := map[string]struct {
		expiresAt time.Time
		want      bool
	}{
		"zero is never expired": {time.Time{}, false},
		"future is not expired": {time.Now().Add(time.Hour), false},
		"past is expired":       {time.Now().Add(-time.Hour), true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &Credentials{ExpiresAt: tc.expiresAt}
			if got := c.Expired(); got != tc.want {
				t.Fatalf("Expired() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDevAuthenticatorAuthenticateAndRefresh(t *testing.T) {
	t.Setenv(EnvJWTSigningKey, "shared-secret")
	authenticator, ok := NewDevAuthenticatorFromEnv()
	if !ok {
		t.Fatal("NewDevAuthenticatorFromEnv() not configured despite key set")
	}

	creds, err := authenticator.Authenticate("me@example.com", "pw", "endpoint:1")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if creds.UserID == "" || creds.AccessToken == "" {
		t.Fatalf("Authenticate() returned empty fields: %+v", creds)
	}

	// The minted token must validate against the same shared secret server-side,
	// and must carry the email claim so the server can provision the user.
	verifier := auth.NewHMACVerifier([]byte("shared-secret"))
	id, err := verifier.Verify(creds.AccessToken)
	if err != nil {
		t.Fatalf("server-side Verify() error = %v", err)
	}
	if id.UserID != creds.UserID {
		t.Fatalf("token subject = %q, want user_id %q", id.UserID, creds.UserID)
	}
	if id.Email != creds.Email {
		t.Fatalf("token email = %q, want %q", id.Email, creds.Email)
	}

	// Refresh must mint a fresh, still-valid token for the same user, preserving
	// the email claim.
	oldToken := creds.AccessToken
	if err := authenticator.Refresh(creds); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if creds.AccessToken == "" {
		t.Fatal("Refresh() produced empty token")
	}
	if id, err := verifier.Verify(creds.AccessToken); err != nil || id.UserID != creds.UserID || id.Email != creds.Email {
		t.Fatalf("refreshed token invalid: id=%+v err=%v", id, err)
	}
	_ = oldToken // tokens may be byte-identical if issued within the same second; subject identity is what matters
}

func TestAuthenticateRequiresEmailAndPassword(t *testing.T) {
	t.Setenv(EnvJWTSigningKey, "shared-secret")
	authenticator, _ := NewDevAuthenticatorFromEnv()

	if _, err := authenticator.Authenticate("", "pw", "e"); err == nil {
		t.Fatal("Authenticate() with empty email succeeded, want error")
	}
	if _, err := authenticator.Authenticate("me@example.com", "", "e"); err == nil {
		t.Fatal("Authenticate() with empty password succeeded, want error")
	}
}

func TestDevUserIDForEmailIsStable(t *testing.T) {
	a := DevUserIDForEmail("me@example.com")
	b := DevUserIDForEmail("me@example.com")
	c := DevUserIDForEmail("other@example.com")
	if a != b {
		t.Fatalf("same email gave different ids: %q vs %q", a, b)
	}
	if a == c {
		t.Fatalf("different emails gave same id: %q", a)
	}
}

func TestNewDevAuthenticatorFromEnvUnsetReturnsFalse(t *testing.T) {
	t.Setenv(EnvJWTSigningKey, "")
	if _, ok := NewDevAuthenticatorFromEnv(); ok {
		t.Fatal("NewDevAuthenticatorFromEnv() reported configured with empty key")
	}
}
