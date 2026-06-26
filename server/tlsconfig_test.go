package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeSelfSignedCert generates a throwaway self-signed certificate/key pair and
// writes them to the given directory, returning their paths.
func writeSelfSignedCert(t *testing.T, dir string) (certFile, keyFile string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certFile = filepath.Join(dir, "cert.pem")
	certPEM, err := os.Create(certFile)
	if err != nil {
		t.Fatalf("create cert file: %v", err)
	}
	if err := pem.Encode(certPEM, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatalf("encode cert: %v", err)
	}
	if err := certPEM.Close(); err != nil {
		t.Fatalf("close cert file: %v", err)
	}

	keyFile = filepath.Join(dir, "key.pem")
	keyPEM, err := os.Create(keyFile)
	if err != nil {
		t.Fatalf("create key file: %v", err)
	}
	keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	if err := pem.Encode(keyPEM, &pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes}); err != nil {
		t.Fatalf("encode key: %v", err)
	}
	if err := keyPEM.Close(); err != nil {
		t.Fatalf("close key file: %v", err)
	}

	return certFile, keyFile
}

func TestTransportCredentialsFromEnv(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := writeSelfSignedCert(t, dir)

	t.Run("no TLS env -> insecure, tlsEnabled false", func(t *testing.T) {
		t.Setenv(EnvTLSCertFile, "")
		t.Setenv(EnvTLSKeyFile, "")

		opt, tlsEnabled, err := TransportCredentialsFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tlsEnabled {
			t.Fatalf("expected tlsEnabled=false when no TLS env set")
		}
		if opt == nil {
			t.Fatalf("expected a non-nil server option")
		}
	})

	t.Run("both cert and key -> TLS enabled", func(t *testing.T) {
		t.Setenv(EnvTLSCertFile, certFile)
		t.Setenv(EnvTLSKeyFile, keyFile)

		opt, tlsEnabled, err := TransportCredentialsFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !tlsEnabled {
			t.Fatalf("expected tlsEnabled=true when cert and key set")
		}
		if opt == nil {
			t.Fatalf("expected a non-nil server option")
		}
	})

	t.Run("only cert set -> error", func(t *testing.T) {
		t.Setenv(EnvTLSCertFile, certFile)
		t.Setenv(EnvTLSKeyFile, "")

		if _, _, err := TransportCredentialsFromEnv(); err == nil {
			t.Fatalf("expected error when only TLS_CERT_FILE is set")
		}
	})

	t.Run("only key set -> error", func(t *testing.T) {
		t.Setenv(EnvTLSCertFile, "")
		t.Setenv(EnvTLSKeyFile, keyFile)

		if _, _, err := TransportCredentialsFromEnv(); err == nil {
			t.Fatalf("expected error when only TLS_KEY_FILE is set")
		}
	})

	t.Run("missing cert file -> error", func(t *testing.T) {
		t.Setenv(EnvTLSCertFile, filepath.Join(dir, "does-not-exist.pem"))
		t.Setenv(EnvTLSKeyFile, keyFile)

		if _, _, err := TransportCredentialsFromEnv(); err == nil {
			t.Fatalf("expected error when the certificate file is missing")
		}
	})
}
