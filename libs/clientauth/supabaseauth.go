package clientauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Supabase Auth (GoTrue) environment variables, read CLI-side to select and
// configure the Supabase authentication mode. The server verifies the resulting
// access tokens with SUPABASE_JWT_SECRET (HS256 legacy secret, Option A of the
// TÂCHE 12 design): the CLI never sees that secret, it only talks to the GoTrue
// REST API over HTTPS.
const (
	// EnvSupabaseURL is the project base URL, e.g. https://<ref>.supabase.co.
	EnvSupabaseURL = "SUPABASE_URL"
	// EnvSupabaseAnonKey is the public anon/publishable key sent as the "apikey"
	// header on every GoTrue request. It is not a secret credential by itself.
	EnvSupabaseAnonKey = "SUPABASE_ANON_KEY"
)

// supabaseHTTPTimeout bounds each GoTrue request so a hung network never blocks
// the CLI indefinitely.
const supabaseHTTPTimeout = 15 * time.Second

// httpDoer is the minimal HTTP client surface SupabaseAuthenticator needs, so
// tests can inject an httptest.Server-backed client without any real Supabase
// project.
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// SupabaseAuthenticator authenticates against Supabase Auth (GoTrue) over HTTPS.
// It implements the Authenticator interface, so the command layer is unchanged:
// only the construction (factory) differs from the local/home DevAuthenticator.
//
// Option A (HS256 legacy) means the access tokens returned here are verified
// server-side by the existing HMACVerifier using SUPABASE_JWT_SECRET; the sub
// claim is the Supabase user UUID and the email claim drives just-in-time user
// provisioning (see server/authinterceptor.go).
type SupabaseAuthenticator struct {
	// baseURL is the GoTrue endpoint base, e.g. https://<ref>.supabase.co/auth/v1.
	baseURL string
	// anonKey is sent as the "apikey" header on every request.
	anonKey string
	client  httpDoer
}

// NewSupabaseAuthenticator builds an authenticator for the given project URL and
// anon key. The projectURL is the bare project URL (https://<ref>.supabase.co);
// the "/auth/v1" GoTrue prefix is appended internally. A nil client uses a
// default http.Client with a timeout.
func NewSupabaseAuthenticator(projectURL, anonKey string, client httpDoer) (*SupabaseAuthenticator, error) {
	projectURL = strings.TrimRight(strings.TrimSpace(projectURL), "/")
	if projectURL == "" {
		return nil, fmt.Errorf("%s is required for Supabase auth", EnvSupabaseURL)
	}
	if strings.TrimSpace(anonKey) == "" {
		return nil, fmt.Errorf("%s is required for Supabase auth", EnvSupabaseAnonKey)
	}
	if client == nil {
		client = &http.Client{Timeout: supabaseHTTPTimeout}
	}
	return &SupabaseAuthenticator{
		baseURL: projectURL + "/auth/v1",
		anonKey: anonKey,
		client:  client,
	}, nil
}

// NewSupabaseAuthenticatorFromEnv builds a SupabaseAuthenticator from
// SUPABASE_URL + SUPABASE_ANON_KEY. It returns (nil, false) when either is
// absent, signalling that the Supabase mode is not configured.
func NewSupabaseAuthenticatorFromEnv() (*SupabaseAuthenticator, bool) {
	url := os.Getenv(EnvSupabaseURL)
	anon := os.Getenv(EnvSupabaseAnonKey)
	if url == "" || anon == "" {
		return nil, false
	}
	a, err := NewSupabaseAuthenticator(url, anon, nil)
	if err != nil {
		return nil, false
	}
	return a, true
}

// gotrueSession is the subset of the GoTrue token/signup response we consume.
type gotrueSession struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	User         struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
}

// gotrueError is the GoTrue error envelope. Newer GoTrue versions use
// {code,error_code,msg}; older ones use {error,error_description}. We surface
// whichever is populated.
type gotrueError struct {
	Msg              string `json:"msg"`
	Message          string `json:"message"`
	ErrorCode        string `json:"error_code"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func (e gotrueError) message() string {
	for _, s := range []string{e.Msg, e.Message, e.ErrorDescription, e.Error, e.ErrorCode} {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// Authenticate logs in with the password grant
// (POST /auth/v1/token?grant_type=password) and maps the GoTrue session onto
// Credentials. signup is a separate endpoint (see SignUp); the command layer
// chooses which to call.
func (s *SupabaseAuthenticator) Authenticate(email, password, endpoint string) (*Credentials, error) {
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if password == "" {
		return nil, fmt.Errorf("password is required")
	}
	body := map[string]string{"email": email, "password": password}
	session, err := s.post("/token?grant_type=password", body, "login")
	if err != nil {
		return nil, err
	}
	return s.credentialsFromSession(session, email, endpoint)
}

// SignUp creates a new account (POST /auth/v1/signup). When the project requires
// email confirmation, GoTrue returns a user without a session (empty
// access_token); in that case we report a clear, actionable error instead of
// writing unusable credentials.
func (s *SupabaseAuthenticator) SignUp(email, password, endpoint string) (*Credentials, error) {
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if password == "" {
		return nil, fmt.Errorf("password is required")
	}
	body := map[string]string{"email": email, "password": password}
	session, err := s.post("/signup", body, "signup")
	if err != nil {
		return nil, err
	}
	if session.AccessToken == "" {
		return nil, fmt.Errorf("account created but no session returned: confirm your email then run 'todo login' (or disable email confirmation in the Supabase dashboard)")
	}
	return s.credentialsFromSession(session, email, endpoint)
}

// Refresh exchanges the stored refresh token for a new session
// (POST /auth/v1/token?grant_type=refresh_token), updating the credentials in
// place. GoTrue rotates refresh tokens, so the new refresh token is persisted.
func (s *SupabaseAuthenticator) Refresh(creds *Credentials) error {
	if creds.RefreshToken == "" {
		return fmt.Errorf("no refresh token available; please log in again")
	}
	body := map[string]string{"refresh_token": creds.RefreshToken}
	session, err := s.post("/token?grant_type=refresh_token", body, "refresh")
	if err != nil {
		return err
	}
	if session.AccessToken == "" {
		return fmt.Errorf("refresh returned no access token; please log in again")
	}
	creds.AccessToken = session.AccessToken
	if session.RefreshToken != "" {
		creds.RefreshToken = session.RefreshToken
	}
	creds.ExpiresAt = expiryFromExpiresIn(session.ExpiresIn)
	if session.User.ID != "" {
		creds.UserID = session.User.ID
	}
	if session.User.Email != "" {
		creds.Email = session.User.Email
	}
	return nil
}

// credentialsFromSession maps a GoTrue session onto Credentials. The user id and
// email come from the response user object; the requested email is used as a
// fallback if GoTrue omits it.
func (s *SupabaseAuthenticator) credentialsFromSession(session *gotrueSession, requestedEmail, endpoint string) (*Credentials, error) {
	if session.AccessToken == "" {
		return nil, fmt.Errorf("authentication succeeded but no access token was returned")
	}
	email := session.User.Email
	if email == "" {
		email = requestedEmail
	}
	return &Credentials{
		Email:        email,
		UserID:       session.User.ID,
		AccessToken:  session.AccessToken,
		RefreshToken: session.RefreshToken,
		ExpiresAt:    expiryFromExpiresIn(session.ExpiresIn),
		Endpoint:     endpoint,
	}, nil
}

// post sends a JSON request to the GoTrue endpoint at path (relative to baseURL),
// attaching the apikey header, and decodes a successful session. Non-2xx
// responses are turned into a clear error including the GoTrue message when
// available. op names the operation for error context (login/signup/refresh).
func (s *SupabaseAuthenticator) post(path string, payload map[string]string, op string) (*gotrueSession, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode %s request: %w", op, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), supabaseHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+path, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("failed to build %s request: %w", op, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", s.anonKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s request to Supabase failed: %w", op, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s response: %w", op, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Supabase %s failed (HTTP %d): %s", op, resp.StatusCode, gotrueErrorMessage(data, resp.Status))
	}

	var session gotrueSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse %s response: %w", op, err)
	}
	return &session, nil
}

// gotrueErrorMessage extracts a human message from a GoTrue error body, falling
// back to the HTTP status text when the body is empty or not JSON.
func gotrueErrorMessage(body []byte, status string) string {
	var ge gotrueError
	if err := json.Unmarshal(body, &ge); err == nil {
		if msg := ge.message(); msg != "" {
			return msg
		}
	}
	if trimmed := strings.TrimSpace(string(body)); trimmed != "" {
		return trimmed
	}
	return status
}

// expiryFromExpiresIn converts GoTrue's expires_in (seconds) to an absolute
// expiry. A non-positive value yields the zero time (treated as "never expired"
// by Credentials.Expired, leaving refresh to the reactive Unauthenticated path).
func expiryFromExpiresIn(expiresIn int) time.Time {
	if expiresIn <= 0 {
		return time.Time{}
	}
	return time.Now().Add(time.Duration(expiresIn) * time.Second)
}
