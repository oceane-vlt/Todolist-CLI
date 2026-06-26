package server

import (
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Environment variables controlling server-side transport security (Phase 4).
const (
	// EnvTLSCertFile is the path to the PEM-encoded server certificate. When set
	// together with EnvTLSKeyFile, the server serves over TLS
	// (credentials.NewServerTLSFromFile), ending the insecure transport.
	EnvTLSCertFile = "TLS_CERT_FILE"

	// EnvTLSKeyFile is the path to the PEM-encoded private key matching
	// EnvTLSCertFile.
	EnvTLSKeyFile = "TLS_KEY_FILE"
)

// TransportCredentialsFromEnv selects the server transport credentials based on
// the environment, returning the dial option and whether TLS is enabled.
//
//   - TLS_CERT_FILE and TLS_KEY_FILE both set -> TLS credentials loaded from
//     those files. This is the path de-risked locally with a self-signed
//     certificate before any remote deployment (docs/implementation-plan.md
//     Phase 4).
//   - neither (or only one) set -> insecure credentials (the pre-Phase-4
//     behaviour), so the default local run keeps working with no configuration.
//
// On a PaaS such as Fly.io, TLS may instead be terminated by the platform
// (docs/target-architecture.md §6.2); in that case the in-process server can
// stay insecure behind the proxy.
//
// Setting only one of the two variables is a misconfiguration and returns an
// error rather than silently falling back to an insecure transport.
func TransportCredentialsFromEnv() (opt grpc.ServerOption, tlsEnabled bool, err error) {
	certFile := os.Getenv(EnvTLSCertFile)
	keyFile := os.Getenv(EnvTLSKeyFile)

	switch {
	case certFile != "" && keyFile != "":
		creds, loadErr := credentials.NewServerTLSFromFile(certFile, keyFile)
		if loadErr != nil {
			return nil, false, fmt.Errorf("loading TLS certificate/key: %w", loadErr)
		}
		return grpc.Creds(creds), true, nil
	case certFile != "" || keyFile != "":
		return nil, false, fmt.Errorf("both %s and %s must be set to enable TLS (got only one)", EnvTLSCertFile, EnvTLSKeyFile)
	default:
		return grpc.Creds(insecure.NewCredentials()), false, nil
	}
}
