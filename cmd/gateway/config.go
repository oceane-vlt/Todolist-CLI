package main

import (
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Environment variables configuring the REST/JSON gateway (Phase 7). The gateway
// is a thin REST <-> gRPC adapter: it dials the existing gRPC server as a client
// and exposes the same seven RPCs over HTTP/JSON for a future web front
// (docs/target-architecture.md §3, docs/implementation-plan.md Phase 7).
const (
	// envHTTPAddr is the address the gateway's HTTP server listens on.
	envHTTPAddr = "TODO_GATEWAY_ADDR"

	// envGRPCEndpoint is the upstream gRPC server the gateway dials. It reuses
	// the same convention as the CLI (TODO_SERVER_ENDPOINT) so a single value
	// configures both.
	envGRPCEndpoint = "TODO_SERVER_ENDPOINT"

	// envGRPCTLS, when truthy, makes the gateway dial the upstream gRPC server
	// over TLS using the system certificate pool (production path against a
	// publicly trusted endpoint such as Fly.io). Unset preserves the insecure
	// local default, mirroring the CLI/server behaviour.
	envGRPCTLS = "TODO_GATEWAY_UPSTREAM_TLS"
)

const (
	// defaultHTTPAddr keeps the gateway loopback-only by default so running it
	// locally never exposes anything publicly without explicit configuration.
	defaultHTTPAddr = "127.0.0.1:8080"

	// defaultGRPCEndpoint matches the CLI/server local default.
	defaultGRPCEndpoint = "127.0.0.1:50051"
)

// httpAddr resolves the HTTP listen address from the environment, falling back
// to the loopback default.
func httpAddr() string {
	if addr := os.Getenv(envHTTPAddr); addr != "" {
		return addr
	}
	return defaultHTTPAddr
}

// grpcEndpoint resolves the upstream gRPC server address from the environment,
// falling back to the local default.
func grpcEndpoint() string {
	if endpoint := os.Getenv(envGRPCEndpoint); endpoint != "" {
		return endpoint
	}
	return defaultGRPCEndpoint
}

// upstreamCredentials selects the transport credentials used to dial the
// upstream gRPC server. TLS when TODO_GATEWAY_UPSTREAM_TLS is truthy (system
// trust pool), insecure otherwise to preserve the local default.
func upstreamCredentials() grpc.DialOption {
	if truthy(os.Getenv(envGRPCTLS)) {
		return grpc.WithTransportCredentials(credentials.NewTLS(nil))
	}
	return grpc.WithTransportCredentials(insecure.NewCredentials())
}

// truthy reports whether an environment value enables a boolean flag.
func truthy(v string) bool {
	switch v {
	case "1", "true", "TRUE", "True", "yes", "on":
		return true
	default:
		return false
	}
}

// validateConfig is a placeholder for future configuration validation; it
// currently only ensures the resolved addresses are non-empty.
func validateConfig() error {
	if httpAddr() == "" {
		return fmt.Errorf("%s resolves to an empty address", envHTTPAddr)
	}
	if grpcEndpoint() == "" {
		return fmt.Errorf("%s resolves to an empty endpoint", envGRPCEndpoint)
	}
	return nil
}
