package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Environment variables controlling client-side transport security and call
// timing (Phase 4).
const (
	// envTLSEnabled, when truthy ("1"/"true"), makes the CLI dial over TLS using
	// the system certificate pool. This is the production path against a publicly
	// trusted endpoint (e.g. Fly.io).
	envTLSEnabled = "TODO_TLS"

	// envTLSCAFile points to a PEM-encoded CA certificate to trust in addition to
	// the system pool. It is what lets the CLI trust a self-signed certificate
	// during local TLS de-risking before deployment (docs/implementation-plan.md
	// Phase 4). Setting it implies TLS.
	envTLSCAFile = "TODO_TLS_CA_FILE"

	// envServerName overrides the server name verified against the certificate.
	// Useful when dialing a self-signed certificate whose SAN does not match the
	// dialed host (e.g. "localhost" while connecting to 127.0.0.1).
	envServerName = "TODO_TLS_SERVER_NAME"

	// envCallTimeout overrides the per-call timeout (Go duration, e.g. "10s").
	envCallTimeout = "TODO_CALL_TIMEOUT"
)

// defaultCallTimeout bounds every RPC so a slow or unreachable remote server
// fails cleanly instead of blocking the CLI indefinitely.
const defaultCallTimeout = 10 * time.Second

// transportCredentials selects the client transport credentials from the
// environment.
//
//   - TODO_TLS_CA_FILE set            -> TLS trusting that CA (local self-signed
//     de-risking path).
//   - TODO_TLS truthy (no CA file)    -> TLS using the system certificate pool
//     (production path against a publicly trusted endpoint).
//   - neither set                     -> insecure credentials, preserving the
//     pre-Phase-4 local default.
func transportCredentials() (grpc.DialOption, error) {
	caFile := os.Getenv(envTLSCAFile)
	serverName := os.Getenv(envServerName)

	if caFile != "" {
		creds, err := credentials.NewClientTLSFromFile(caFile, serverName)
		if err != nil {
			return nil, fmt.Errorf("loading TLS CA certificate: %w", err)
		}
		return grpc.WithTransportCredentials(creds), nil
	}

	if tlsEnabled() {
		// System trust pool; ServerName left empty so it is derived from the
		// dialed endpoint.
		creds := credentials.NewTLS(nil)
		return grpc.WithTransportCredentials(creds), nil
	}

	return grpc.WithTransportCredentials(insecure.NewCredentials()), nil
}

// tlsEnabled reports whether TODO_TLS requests a TLS transport.
func tlsEnabled() bool {
	v, err := strconv.ParseBool(os.Getenv(envTLSEnabled))
	return err == nil && v
}

// callTimeout resolves the per-call timeout from the environment, falling back
// to defaultCallTimeout when unset or invalid.
func callTimeout() time.Duration {
	if raw := os.Getenv(envCallTimeout); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			return d
		}
	}
	return defaultCallTimeout
}

// timeoutUnaryInterceptor bounds every RPC with callTimeout() so that a slow or
// unreachable remote server fails cleanly (DeadlineExceeded) instead of blocking
// the CLI. If the caller already set a (shorter) deadline, it is left untouched.
func timeoutUnaryInterceptor(
	ctx context.Context,
	method string,
	req, reply any,
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return invoker(ctx, method, req, reply, cc, opts...)
	}
	ctx, cancel := context.WithTimeout(ctx, callTimeout())
	defer cancel()
	return invoker(ctx, method, req, reply, cc, opts...)
}
