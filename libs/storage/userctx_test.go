package storage

import (
	"context"
	"testing"
)

func TestUserIDFromContext(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		if id, ok := UserIDFromContext(context.Background()); ok || id != "" {
			t.Fatalf("expected no user id, got %q (ok=%v)", id, ok)
		}
	})

	t.Run("present", func(t *testing.T) {
		ctx := WithUserID(context.Background(), "user-123")
		id, ok := UserIDFromContext(ctx)
		if !ok || id != "user-123" {
			t.Fatalf("expected user-123, got %q (ok=%v)", id, ok)
		}
	})

	t.Run("empty treated as absent", func(t *testing.T) {
		ctx := WithUserID(context.Background(), "")
		if id, ok := UserIDFromContext(ctx); ok || id != "" {
			t.Fatalf("empty user id should be reported absent, got %q (ok=%v)", id, ok)
		}
	})
}

// TestPgStoreUserIDResolution checks the per-request resolution: context value
// wins, otherwise the store default (placeholder) is used. No DB needed.
func TestPgStoreUserIDResolution(t *testing.T) {
	s := &PgStore{defaultUserID: PlaceholderUserID}

	if got := s.userID(context.Background()); got != PlaceholderUserID {
		t.Fatalf("expected fallback to placeholder, got %q", got)
	}
	ctx := WithUserID(context.Background(), "real-user")
	if got := s.userID(ctx); got != "real-user" {
		t.Fatalf("expected context user, got %q", got)
	}
}
