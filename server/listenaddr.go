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

// IsLoopbackListenAddr reports whether addr binds the server to the loopback
// interface only (so it is unreachable from other hosts). It is used to gate
// the unauthenticated dev mode: running with no authentication is only safe
// when the server cannot be reached from outside the machine.
//
// An address is considered loopback only when its host resolves
// unambiguously to a loopback address:
//
//   - an explicit loopback IP literal (127.0.0.0/8, ::1);
//   - the hostname "localhost".
//
// Anything else is treated as NON-loopback (fail-safe), including:
//
//   - the unspecified/wildcard addresses "0.0.0.0" and "::" (bind all
//     interfaces, the PaaS form);
//   - an empty host (e.g. ":50051"), which binds all interfaces;
//   - any other hostname or interface IP;
//   - a malformed address we cannot parse.
//
// Defaulting unknown/unparseable hosts to non-loopback is deliberate: it errs
// toward refusing the insecure no-auth mode rather than silently allowing a
// public bind without authentication.
func IsLoopbackListenAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// No port (or otherwise malformed): treat the whole string as the host so
		// a bare "127.0.0.1" or "localhost" still resolves, and anything we cannot
		// classify falls through to non-loopback below.
		host = addr
	}
	if host == "" {
		// ":50051" / empty host binds all interfaces -> not loopback.
		return false
	}
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	// Unknown hostname we cannot resolve to a loopback IP -> fail safe.
	return false
}
