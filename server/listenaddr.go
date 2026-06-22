package server

import (
	"net"
	"os"
)

// Environment variables controlling the server listen address (Phase 6).
const (
	// EnvListenAddr overrides the full listen address (host:port). When set it
	// takes precedence over EnvPort. Use it to bind a specific interface, e.g.
	// "0.0.0.0:50051" to accept remote connections, or keep the default
	// loopback-only bind for local development.
	EnvListenAddr = "TODO_LISTEN_ADDR"

	// EnvPort is the listen port provided by the PaaS (Fly.io sets PORT). When
	// set (and TODO_LISTEN_ADDR is not), the server binds 0.0.0.0:$PORT so it is
	// reachable from outside the container (docs/target-architecture.md §6.2).
	EnvPort = "PORT"

	// defaultListenAddr is the pre-Phase-6 behaviour: loopback only, so the
	// default local run keeps working unchanged with no configuration.
	defaultListenAddr = "127.0.0.1:50051"
)

// ListenAddrFromEnv resolves the address the gRPC server should listen on.
//
//   - TODO_LISTEN_ADDR set -> used verbatim (full host:port).
//   - else PORT set -> 0.0.0.0:$PORT, the form expected when hosting on a PaaS
//     such as Fly.io where the platform injects PORT and routes external
//     traffic to the container's bound port.
//   - else -> 127.0.0.1:50051, the original loopback-only default so local
//     development is unchanged with no configuration.
func ListenAddrFromEnv() string {
	if addr := os.Getenv(EnvListenAddr); addr != "" {
		return addr
	}
	if port := os.Getenv(EnvPort); port != "" {
		return net.JoinHostPort("0.0.0.0", port)
	}
	return defaultListenAddr
}
