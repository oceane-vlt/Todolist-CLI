package server

import (
	"context"

	"github.com/oceane-vlt/todolist/libs/storage"
	"google.golang.org/grpc"
)

// DevUserIDInterceptor returns a unary server interceptor that injects a fixed
// user identity into the request context (storage.WithUserID), so the storage
// layer scopes every operation to that user_id (Phase 2 isolation).
//
// This is a development/transition stand-in: the user_id is configured at
// startup rather than derived from a credential. In Phase 3 this interceptor is
// replaced by an authentication interceptor that validates the JWT and injects
// the real, per-request user_id through the same storage.WithUserID seam — no
// change is needed in the storage layer or the handlers.
func DevUserIDInterceptor(userID string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(storage.WithUserID(ctx, userID), req)
	}
}
