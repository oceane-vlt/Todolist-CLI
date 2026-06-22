package server

import (
	"context"
	"strings"

	"github.com/oceane-vlt/todolist/libs/auth"
	"github.com/oceane-vlt/todolist/libs/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// authMetadataKey is the gRPC metadata key carrying the bearer token. gRPC
// lowercases all metadata keys, so we match against the lowercase form.
const authMetadataKey = "authorization"

const bearerPrefix = "Bearer "

// UserProvisioner provisions the authenticated user in the persistent store
// before the request runs (just-in-time provisioning). It is satisfied by
// *storage.PgStore.EnsureUser. The server wires a provisioner only when the
// store is Postgres-backed; the JSON backend has no users table and needs none.
//
// Keeping this interface in the server package (rather than widening the storage
// Store interface) is what stops the 7 business RPCs from gaining a user-table
// concern, while still letting the interceptor trigger provisioning.
type UserProvisioner interface {
	EnsureUser(ctx context.Context, userID, email string) error
}

// AuthInterceptor returns a unary server interceptor that authenticates the
// caller from the "authorization: Bearer <JWT>" metadata, then injects the
// resulting user identity into the context via storage.WithUserID — the same
// seam the Phase 2 dev interceptor used. The storage layer therefore scopes
// every request to the authenticated user with no further change.
//
// When provisioner is non-nil and the validated token carries an email claim,
// the interceptor provisions the user (best-effort: a token without an email is
// not provisioned, so legacy sub-only tokens keep working) BEFORE the handler
// runs, so the first write by a real authenticated user does not violate the
// lists.user_id foreign key. A provisioning failure is surfaced as a gRPC error.
//
// Identity comes exclusively from the validated token's subject and email
// claims, never from a client-supplied field (docs/target-architecture.md
// §4.1): a client cannot impersonate another user. Missing, malformed or
// expired tokens are rejected with codes.Unauthenticated, which the CLI uses as
// the trigger to refresh and replay the call (docs/target-architecture.md §5.3).
func AuthInterceptor(verifier auth.Verifier, provisioner UserProvisioner) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		token, err := bearerFromContext(ctx)
		if err != nil {
			return nil, err
		}

		identity, err := verifier.Verify(token)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
		}

		if provisioner != nil && identity.Email != "" {
			if err := provisioner.EnsureUser(ctx, identity.UserID, identity.Email); err != nil {
				return nil, status.Errorf(codes.Internal, "failed to provision user: %v", err)
			}
		}

		return handler(storage.WithUserID(ctx, identity.UserID), req)
	}
}

// bearerFromContext extracts the bearer token from the incoming gRPC metadata,
// returning an Unauthenticated status error if it is absent or malformed.
func bearerFromContext(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing authentication metadata")
	}
	values := md.Get(authMetadataKey)
	if len(values) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization header")
	}
	header := values[0]
	if !strings.HasPrefix(header, bearerPrefix) {
		return "", status.Error(codes.Unauthenticated, "authorization header must use the Bearer scheme")
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, bearerPrefix))
	if token == "" {
		return "", status.Error(codes.Unauthenticated, "empty bearer token")
	}
	return token, nil
}
