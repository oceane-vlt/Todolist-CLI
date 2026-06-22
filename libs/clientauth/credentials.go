// Package clientauth manages the CLI's authentication state (Phase 3 of
// docs/implementation-plan.md): the credentials file, token issuance in the
// local/home dev mode, and the refresh logic used to replay a call after an
// Unauthenticated response.
//
// Token storage follows docs/target-architecture.md §5.2: credentials live in
// ~/.config/todolist/credentials.json with permissions 0600 and carry the
// access token, refresh token, expiry and the server endpoint.
//
// In the Supabase target the access/refresh tokens come from Supabase Auth over
// HTTPS; that exchange is an external integration documented for the deployment
// step. To de-risk locally first (as the plan requires), this package also ships
// a self-contained home/dev mode that mints HS256 tokens with the shared
// JWT_SIGNING_KEY, so login/refresh and the whole metadata flow are exercised
// end to end without any external account.
package clientauth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CredentialsFileName is the file (under the todolist config dir) holding the
// CLI tokens.
const CredentialsFileName = "credentials.json"

// Credentials is the persisted authentication state.
type Credentials struct {
	Email        string    `json:"email"`
	UserID       string    `json:"user_id"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	Endpoint     string    `json:"endpoint"`
}

// Expired reports whether the access token is past (or within a small skew of)
// its expiry, in which case it should be refreshed before use.
func (c *Credentials) Expired() bool {
	if c.ExpiresAt.IsZero() {
		return false
	}
	const skew = 30 * time.Second
	return time.Now().Add(skew).After(c.ExpiresAt)
}

// CredentialsPath returns the absolute path of the credentials file, creating
// the parent config directory (~/.config/todolist) if needed.
func CredentialsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	configDir := filepath.Join(home, ".config", "todolist")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}
	return filepath.Join(configDir, CredentialsFileName), nil
}

// Load reads the credentials from the given path. It returns os.ErrNotExist
// (wrapped) when the file is absent, so callers can prompt the user to log in.
func Load(path string) (*Credentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return &creds, nil
}

// Save writes the credentials to path with 0600 permissions (owner read/write
// only), matching docs/target-architecture.md §5.2. It rewrites the file
// atomically-ish via a fresh write and enforces the mode even if the file
// already existed with looser permissions.
func Save(path string, creds *Credentials) error {
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode credentials: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	// Defensively re-apply 0600 in case the file pre-existed with other perms.
	if err := os.Chmod(path, 0600); err != nil {
		return fmt.Errorf("failed to set permissions on %s: %w", path, err)
	}
	return nil
}

// Delete removes the credentials file (logout). A missing file is not an error.
func Delete(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove %s: %w", path, err)
	}
	return nil
}
