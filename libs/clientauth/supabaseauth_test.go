package clientauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestSupabaseAuthenticator wires a SupabaseAuthenticator to an
// httptest.Server so the GoTrue REST contract is exercised without any real
// Supabase project. The server's client (which already targets the test server)
// is injected as the httpDoer.
func newTestSupabaseAuthenticator(t *testing.T, srv *httptest.Server) *SupabaseAuthenticator {
	t.Helper()
	a, err := NewSupabaseAuthenticator(srv.URL, "anon-key", srv.Client())
	if err != nil {
		t.Fatalf("NewSupabaseAuthenticator() error = %v", err)
	}
	return a
}

func TestSupabaseAuthenticateLoginSuccess(t *testing.T) {
	var gotPath, gotQuery, gotAPIKey, gotContentType string
	var gotBody map[string]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAPIKey = r.Header.Get("apikey")
		gotContentType = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"access_token": "supabase-access",
			"refresh_token": "supabase-refresh",
			"token_type": "bearer",
			"expires_in": 3600,
			"user": {"id": "user-uuid-123", "email": "me@example.com"}
		}`))
	}))
	defer srv.Close()

	a := newTestSupabaseAuthenticator(t, srv)
	creds, err := a.Authenticate("me@example.com", "pw", "127.0.0.1:50051")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	if gotPath != "/auth/v1/token" {
		t.Errorf("login path = %q, want /auth/v1/token", gotPath)
	}
	if gotQuery != "grant_type=password" {
		t.Errorf("login query = %q, want grant_type=password", gotQuery)
	}
	if gotAPIKey != "anon-key" {
		t.Errorf("apikey header = %q, want anon-key", gotAPIKey)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotBody["email"] != "me@example.com" || gotBody["password"] != "pw" {
		t.Errorf("login body = %+v, want email/password", gotBody)
	}

	if creds.AccessToken != "supabase-access" {
		t.Errorf("AccessToken = %q, want supabase-access", creds.AccessToken)
	}
	if creds.RefreshToken != "supabase-refresh" {
		t.Errorf("RefreshToken = %q, want supabase-refresh", creds.RefreshToken)
	}
	if creds.UserID != "user-uuid-123" {
		t.Errorf("UserID = %q, want user-uuid-123 (=sub)", creds.UserID)
	}
	if creds.Email != "me@example.com" {
		t.Errorf("Email = %q, want me@example.com", creds.Email)
	}
	if creds.Endpoint != "127.0.0.1:50051" {
		t.Errorf("Endpoint = %q, want 127.0.0.1:50051", creds.Endpoint)
	}
	// expires_in=3600 -> a future, non-zero expiry.
	if creds.ExpiresAt.IsZero() || creds.ExpiresAt.Before(time.Now()) {
		t.Errorf("ExpiresAt = %v, want a future time", creds.ExpiresAt)
	}
}

func TestSupabaseAuthenticateFallsBackToRequestedEmail(t *testing.T) {
	// GoTrue may omit user.email; the requested email must populate Credentials so
	// the server-side just-in-time provisioning still has an email.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"a","refresh_token":"r","expires_in":3600,"user":{"id":"uid"}}`))
	}))
	defer srv.Close()

	a := newTestSupabaseAuthenticator(t, srv)
	creds, err := a.Authenticate("me@example.com", "pw", "e")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if creds.Email != "me@example.com" {
		t.Errorf("Email = %q, want fallback to requested me@example.com", creds.Email)
	}
}

func TestSupabaseAuthenticateUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"Invalid login credentials"}`))
	}))
	defer srv.Close()

	a := newTestSupabaseAuthenticator(t, srv)
	_, err := a.Authenticate("me@example.com", "wrong", "e")
	if err == nil {
		t.Fatal("Authenticate() with bad credentials succeeded, want error")
	}
	if !strings.Contains(err.Error(), "Invalid login credentials") {
		t.Errorf("error = %v, want it to surface the GoTrue message", err)
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error = %v, want it to include the HTTP status code", err)
	}
}

func TestSupabaseAuthenticate401SurfacesNewEnvelope(t *testing.T) {
	// Newer GoTrue uses {code,error_code,msg}.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":401,"error_code":"invalid_credentials","msg":"bad creds"}`))
	}))
	defer srv.Close()

	a := newTestSupabaseAuthenticator(t, srv)
	_, err := a.Authenticate("me@example.com", "pw", "e")
	if err == nil {
		t.Fatal("Authenticate() got nil error on 401")
	}
	if !strings.Contains(err.Error(), "bad creds") {
		t.Errorf("error = %v, want the msg field surfaced", err)
	}
}

func TestSupabaseAuthenticateRequiresEmailAndPassword(t *testing.T) {
	// No server should be hit; use one that fails the test if called.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server hit despite missing input: %s", r.URL)
	}))
	defer srv.Close()

	a := newTestSupabaseAuthenticator(t, srv)
	if _, err := a.Authenticate("", "pw", "e"); err == nil {
		t.Error("Authenticate() with empty email succeeded, want error")
	}
	if _, err := a.Authenticate("me@example.com", "", "e"); err == nil {
		t.Error("Authenticate() with empty password succeeded, want error")
	}
}

func TestSupabaseSignUpSuccess(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{
			"access_token": "new-access",
			"refresh_token": "new-refresh",
			"expires_in": 3600,
			"user": {"id": "new-uuid", "email": "new@example.com"}
		}`))
	}))
	defer srv.Close()

	a := newTestSupabaseAuthenticator(t, srv)
	creds, err := a.SignUp("new@example.com", "pw", "e")
	if err != nil {
		t.Fatalf("SignUp() error = %v", err)
	}
	if gotPath != "/auth/v1/signup" {
		t.Errorf("signup path = %q, want /auth/v1/signup", gotPath)
	}
	if creds.AccessToken != "new-access" || creds.UserID != "new-uuid" {
		t.Errorf("SignUp() credentials = %+v, want new-access / new-uuid", creds)
	}
}

func TestSupabaseSignUpEmailConfirmationRequired(t *testing.T) {
	// When email confirmation is on, GoTrue returns a user but no session
	// (empty access_token). That must become a clear, actionable error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"user":{"id":"uid","email":"new@example.com"}}`))
	}))
	defer srv.Close()

	a := newTestSupabaseAuthenticator(t, srv)
	_, err := a.SignUp("new@example.com", "pw", "e")
	if err == nil {
		t.Fatal("SignUp() without a session succeeded, want actionable error")
	}
	if !strings.Contains(err.Error(), "confirm your email") {
		t.Errorf("error = %v, want it to mention confirming the email", err)
	}
}

func TestSupabaseSignUp400(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"msg":"User already registered"}`))
	}))
	defer srv.Close()

	a := newTestSupabaseAuthenticator(t, srv)
	_, err := a.SignUp("dup@example.com", "pw", "e")
	if err == nil {
		t.Fatal("SignUp() got nil error on 400")
	}
	if !strings.Contains(err.Error(), "User already registered") {
		t.Errorf("error = %v, want the GoTrue msg surfaced", err)
	}
}

func TestSupabaseRefreshSuccessRotatesToken(t *testing.T) {
	var gotPath, gotQuery string
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{
			"access_token": "rotated-access",
			"refresh_token": "rotated-refresh",
			"expires_in": 3600,
			"user": {"id": "uid", "email": "me@example.com"}
		}`))
	}))
	defer srv.Close()

	a := newTestSupabaseAuthenticator(t, srv)
	creds := &Credentials{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		UserID:       "uid",
		Email:        "me@example.com",
	}
	if err := a.Refresh(creds); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if gotPath != "/auth/v1/token" {
		t.Errorf("refresh path = %q, want /auth/v1/token", gotPath)
	}
	if gotQuery != "grant_type=refresh_token" {
		t.Errorf("refresh query = %q, want grant_type=refresh_token", gotQuery)
	}
	if gotBody["refresh_token"] != "old-refresh" {
		t.Errorf("refresh body refresh_token = %q, want old-refresh", gotBody["refresh_token"])
	}
	if creds.AccessToken != "rotated-access" {
		t.Errorf("AccessToken = %q, want rotated-access", creds.AccessToken)
	}
	// Rotation: the new refresh token must be persisted.
	if creds.RefreshToken != "rotated-refresh" {
		t.Errorf("RefreshToken = %q, want rotated-refresh (rotation persisted)", creds.RefreshToken)
	}
}

func TestSupabaseRefreshKeepsOldTokenWhenNoneReturned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"fresh","expires_in":3600}`))
	}))
	defer srv.Close()

	a := newTestSupabaseAuthenticator(t, srv)
	creds := &Credentials{AccessToken: "old", RefreshToken: "keep-me"}
	if err := a.Refresh(creds); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if creds.RefreshToken != "keep-me" {
		t.Errorf("RefreshToken = %q, want it preserved when none returned", creds.RefreshToken)
	}
	if creds.AccessToken != "fresh" {
		t.Errorf("AccessToken = %q, want fresh", creds.AccessToken)
	}
}

func TestSupabaseRefreshWithoutTokenErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server hit despite missing refresh token: %s", r.URL)
	}))
	defer srv.Close()

	a := newTestSupabaseAuthenticator(t, srv)
	if err := a.Refresh(&Credentials{}); err == nil {
		t.Fatal("Refresh() with no refresh token succeeded, want error")
	}
}

func TestSupabaseRefresh401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"refresh token expired"}`))
	}))
	defer srv.Close()

	a := newTestSupabaseAuthenticator(t, srv)
	err := a.Refresh(&Credentials{RefreshToken: "expired"})
	if err == nil {
		t.Fatal("Refresh() got nil error on 401")
	}
	if !strings.Contains(err.Error(), "refresh token expired") {
		t.Errorf("error = %v, want the GoTrue message surfaced", err)
	}
}

func TestNewSupabaseAuthenticatorValidation(t *testing.T) {
	if _, err := NewSupabaseAuthenticator("", "anon", nil); err == nil {
		t.Error("NewSupabaseAuthenticator() with empty URL succeeded, want error")
	}
	if _, err := NewSupabaseAuthenticator("https://x.supabase.co", "", nil); err == nil {
		t.Error("NewSupabaseAuthenticator() with empty anon key succeeded, want error")
	}
}

func TestNewSupabaseAuthenticatorTrimsTrailingSlash(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"access_token":"a","refresh_token":"r","expires_in":3600,"user":{"id":"u","email":"e@e.com"}}`))
	}))
	defer srv.Close()

	// Trailing slash on the project URL must not produce a doubled-slash path.
	a, err := NewSupabaseAuthenticator(srv.URL+"/", "anon", srv.Client())
	if err != nil {
		t.Fatalf("NewSupabaseAuthenticator() error = %v", err)
	}
	if _, err := a.Authenticate("e@e.com", "pw", "e"); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if gotPath != "/auth/v1/token" {
		t.Errorf("path = %q, want /auth/v1/token (no doubled slash)", gotPath)
	}
}
