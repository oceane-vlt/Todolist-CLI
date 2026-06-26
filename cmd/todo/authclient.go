package main

import (
	"context"
	"os"

	"github.com/oceane-vlt/todolist/libs/clientauth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// authUnaryInterceptor is a client-side unary interceptor that attaches the
// stored access token as "authorization: Bearer <token>" metadata and, on an
// Unauthenticated reply, transparently refreshes the token and replays the call
// once (docs/target-architecture.md §5.3).
//
// When no credentials exist (the user never logged in), the call is sent as-is:
// against a dev server with auth disabled it still works, and against an
// auth-enabled server it returns Unauthenticated, prompting the user to log in.
func authUnaryInterceptor(
	ctx context.Context,
	method string,
	req, reply any,
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	credsPath, err := clientauth.CredentialsPath()
	if err != nil {
		return err
	}

	creds, loadErr := clientauth.Load(credsPath)
	if loadErr != nil {
		// No (or unreadable) credentials: proceed unauthenticated. The server
		// decides whether that is acceptable.
		if os.IsNotExist(loadErr) {
			return invoker(ctx, method, req, reply, cc, opts...)
		}
		return loadErr
	}

	// Proactively refresh if the access token is known to be expired.
	if creds.Expired() {
		if refreshErr := refreshAndSave(credsPath, creds); refreshErr != nil {
			return refreshErr
		}
	}

	authCtx := withBearer(ctx, creds.AccessToken)
	err = invoker(authCtx, method, req, reply, cc, opts...)
	if status.Code(err) != codes.Unauthenticated {
		return err
	}

	// Reactive refresh: the server rejected the token; try once more with a
	// fresh one before surfacing the error to the user.
	if refreshErr := refreshAndSave(credsPath, creds); refreshErr != nil {
		return err // keep the original Unauthenticated error
	}
	return invoker(withBearer(ctx, creds.AccessToken), method, req, reply, cc, opts...)
}

// withBearer returns ctx with the access token attached as bearer metadata.
func withBearer(ctx context.Context, accessToken string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+accessToken)
}

// refreshAndSave refreshes the access token in place and persists it. The
// refresh mechanism is selected the same way login is (home/dev mode here;
// Supabase HTTPS in the target).
func refreshAndSave(credsPath string, creds *clientauth.Credentials) error {
	authenticator, err := clientauth.NewAuthenticatorFromEnv()
	if err != nil {
		return status.Error(codes.Unauthenticated, "session expired and no refresh mode configured; run 'todo login'")
	}
	if err := authenticator.Refresh(creds); err != nil {
		return err
	}
	return clientauth.Save(credsPath, creds)
}
