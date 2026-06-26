package storage

import "context"

// userIDContextKey is the unexported context key under which the authenticated
// user identity is carried. Using an unexported named type avoids collisions
// with keys set by other packages (Go context best practice).
type userIDContextKey struct{}

// WithUserID returns a copy of ctx carrying the given user identity. This is the
// Phase 2 seam (docs/implementation-plan.md): a request-scoped user_id is
// propagated through the context so the storage layer can isolate data per user
// (WHERE user_id = $1). In Phase 2 the value is injected by a dev interceptor;
// in Phase 3 the same mechanism is fed by the JWT auth interceptor, with no
// further signature churn.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDContextKey{}, userID)
}

// UserIDFromContext returns the user identity carried by ctx, if any. The second
// result reports whether a (non-empty) user_id was present.
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDContextKey{}).(string)
	if !ok || id == "" {
		return "", false
	}
	return id, true
}
