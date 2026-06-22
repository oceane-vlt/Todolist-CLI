package clientauth

import "fmt"

// ErrNoAuthMode is returned by NewAuthenticatorFromEnv when neither the Supabase
// nor the local/home mode is configured. The command layer turns it into a
// guided message pointing the user at the two configuration paths.
var ErrNoAuthMode = fmt.Errorf("no authentication mode configured: set %s+%s for Supabase, or %s for local/dev",
	EnvSupabaseURL, EnvSupabaseAnonKey, EnvJWTSigningKey)

// SignUpAuthenticator is an Authenticator that also supports account creation as
// a distinct operation (Supabase has separate signup and login endpoints). The
// local/home DevAuthenticator does not implement it: there signup and login are
// the same operation, so the command layer falls back to Authenticate.
type SignUpAuthenticator interface {
	Authenticator
	// SignUp creates a new account and returns fresh credentials.
	SignUp(email, password, endpoint string) (*Credentials, error)
}

// NewAuthenticatorFromEnv selects the authentication mode from the environment,
// matching the TÂCHE 12 design precedence:
//   - SUPABASE_URL + SUPABASE_ANON_KEY present -> Supabase Auth (GoTrue HTTPS).
//   - else JWT_SIGNING_KEY present -> local/home DevAuthenticator (HS256).
//   - else ErrNoAuthMode.
//
// Supabase takes precedence so that, once a project is configured, the CLI uses
// the real identity provider even if a leftover JWT_SIGNING_KEY is still set.
func NewAuthenticatorFromEnv() (Authenticator, error) {
	if a, ok := NewSupabaseAuthenticatorFromEnv(); ok {
		return a, nil
	}
	if a, ok := NewDevAuthenticatorFromEnv(); ok {
		return a, nil
	}
	return nil, ErrNoAuthMode
}
