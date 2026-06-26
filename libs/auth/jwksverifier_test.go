package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// These tests exercise the JWKSVerifier end to end WITHOUT any real Supabase
// project: a test EC P-256 key is generated, its public half is published as a
// JWKS via httptest, and tokens are signed locally with the matching private
// key. This validates the ES256 accept path, the reject paths (unknown kid,
// expired, bad signature, alg=none, HS256-on-asymmetric-key), key rotation
// (refetch on unknown kid, rate-limited), and stale-on-error resilience.

const testKID = "test-kid-1"

// ecKeyMaterial is a generated EC P-256 key plus its JWKS-encoded public form.
type ecKeyMaterial struct {
	priv *ecdsa.PrivateKey
	kid  string
}

func newECKey(t *testing.T, kid string) ecKeyMaterial {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	return ecKeyMaterial{priv: priv, kid: kid}
}

// jwk renders the public key as a single JWKS entry (EC P-256, ES256).
func (k ecKeyMaterial) jwk() map[string]string {
	return map[string]string{
		"kty": "EC",
		"crv": "P-256",
		"kid": k.kid,
		"alg": "ES256",
		"use": "sig",
		"x":   b64Coord(k.priv.X),
		"y":   b64Coord(k.priv.Y),
	}
}

// b64Coord encodes an EC coordinate left-padded to the P-256 32-byte field size
// as base64url (no padding), matching how Supabase publishes JWK coordinates.
func b64Coord(c *big.Int) string {
	const size = 32
	b := c.Bytes()
	if len(b) < size {
		padded := make([]byte, size)
		copy(padded[size-len(b):], b)
		b = padded
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// jwksJSON builds a JWKS document JSON for the given keys.
func jwksJSON(t *testing.T, keys ...ecKeyMaterial) []byte {
	t.Helper()
	entries := make([]map[string]string, 0, len(keys))
	for _, k := range keys {
		entries = append(entries, k.jwk())
	}
	doc := map[string]any{"keys": entries}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal(jwks) error = %v", err)
	}
	return b
}

// signES256 signs claims with the EC private key, setting the kid header.
func signES256(t *testing.T, k ecKeyMaterial, claims jwt.Claims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tok.Header["kid"] = k.kid
	signed, err := tok.SignedString(k.priv)
	if err != nil {
		t.Fatalf("SignedString(ES256) error = %v", err)
	}
	return signed
}

// validClaims returns claims with a future expiry, a subject, and an email.
func validClaims(sub, email string) tokenClaims {
	return tokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   sub,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Email: email,
	}
}

// jwksServer serves a fixed JWKS body and counts hits, so tests can assert
// caching / refetch behaviour. The body can be swapped under lock to simulate
// rotation, and failNext forces an error response.
type jwksServer struct {
	srv      *httptest.Server
	hits     atomic.Int64
	mu       chan struct{} // 1-slot mutex-ish guard for body swap
	body     []byte
	failCode int // when non-zero, respond with this status instead of body
}

func newJWKSServer(body []byte) *jwksServer {
	js := &jwksServer{body: body, mu: make(chan struct{}, 1)}
	js.mu <- struct{}{}
	js.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		js.hits.Add(1)
		<-js.mu
		code := js.failCode
		b := js.body
		js.mu <- struct{}{}
		if code != 0 {
			w.WriteHeader(code)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	return js
}

func (js *jwksServer) setBody(b []byte) {
	<-js.mu
	js.body = b
	js.mu <- struct{}{}
}

func (js *jwksServer) setFailCode(code int) {
	<-js.mu
	js.failCode = code
	js.mu <- struct{}{}
}

func (js *jwksServer) close() { js.srv.Close() }

// newVerifier builds a JWKSVerifier pointed at the test server, using its
// client so requests stay in-process.
func newVerifier(t *testing.T, js *jwksServer, opts ...JWKSOption) *JWKSVerifier {
	t.Helper()
	allOpts := append([]JWKSOption{WithHTTPClient(js.srv.Client())}, opts...)
	v, err := NewJWKSVerifier(js.srv.URL, allOpts...)
	if err != nil {
		t.Fatalf("NewJWKSVerifier() error = %v", err)
	}
	return v
}

func TestJWKSVerifyAcceptsValidES256Token(t *testing.T) {
	key := newECKey(t, testKID)
	js := newJWKSServer(jwksJSON(t, key))
	defer js.close()
	v := newVerifier(t, js)

	token := signES256(t, key, validClaims("user-123", "user@example.com"))
	got, err := v.Verify(token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if got.UserID != "user-123" {
		t.Fatalf("Verify() userID = %q, want %q", got.UserID, "user-123")
	}
	if got.Email != "user@example.com" {
		t.Fatalf("Verify() email = %q, want %q", got.Email, "user@example.com")
	}
}

func TestJWKSVerifyToleratesMissingEmailClaim(t *testing.T) {
	key := newECKey(t, testKID)
	js := newJWKSServer(jwksJSON(t, key))
	defer js.close()
	v := newVerifier(t, js)

	token := signES256(t, key, validClaims("user-123", ""))
	got, err := v.Verify(token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if got.Email != "" {
		t.Fatalf("Verify() email = %q, want empty", got.Email)
	}
}

func TestJWKSVerifyRejectsUnknownKID(t *testing.T) {
	key := newECKey(t, testKID)
	js := newJWKSServer(jwksJSON(t, key))
	defer js.close()
	v := newVerifier(t, js)

	// Sign with a key whose kid is NOT in the published JWKS. A refetch is
	// triggered but still finds nothing -> reject.
	other := newECKey(t, "other-kid")
	token := signES256(t, other, validClaims("user-123", ""))
	if _, err := v.Verify(token); err == nil {
		t.Fatal("Verify() accepted a token with an unknown kid, want error")
	}
}

func TestJWKSVerifyRejectsExpiredToken(t *testing.T) {
	key := newECKey(t, testKID)
	js := newJWKSServer(jwksJSON(t, key))
	defer js.close()
	v := newVerifier(t, js)

	claims := tokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-123",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)), // expired
		},
	}
	token := signES256(t, key, claims)
	if _, err := v.Verify(token); err == nil {
		t.Fatal("Verify() accepted an expired token, want error")
	}
}

func TestJWKSVerifyRejectsBadSignature(t *testing.T) {
	key := newECKey(t, testKID)
	js := newJWKSServer(jwksJSON(t, key))
	defer js.close()
	v := newVerifier(t, js)

	// Sign with a DIFFERENT private key but reuse the published kid, so the key
	// lookup succeeds yet the signature fails to verify against the JWKS key.
	imposter := newECKey(t, testKID)
	token := signES256(t, imposter, validClaims("user-123", ""))
	if _, err := v.Verify(token); err == nil {
		t.Fatal("Verify() accepted a token with an invalid signature, want error")
	}
}

func TestJWKSVerifyRejectsNoneAlg(t *testing.T) {
	key := newECKey(t, testKID)
	js := newJWKSServer(jwksJSON(t, key))
	defer js.close()
	v := newVerifier(t, js)

	tok := jwt.NewWithClaims(jwt.SigningMethodNone, validClaims("user-123", ""))
	tok.Header["kid"] = testKID
	signed, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("SignedString(none) error = %v", err)
	}
	if _, err := v.Verify(signed); err == nil {
		t.Fatal("Verify() accepted an alg=none token, want error")
	}
}

func TestJWKSVerifyRejectsHS256OnAsymmetricKey(t *testing.T) {
	// The classic algorithm-confusion attack: an attacker takes the public key
	// (which is, well, public) and signs an HS256 token using the public key
	// bytes as the HMAC secret. The verifier must reject HS256 outright before
	// any key lookup. We use the raw public coordinates as the "secret".
	key := newECKey(t, testKID)
	js := newJWKSServer(jwksJSON(t, key))
	defer js.close()
	v := newVerifier(t, js)

	pubBytes := elliptic.Marshal(key.priv.Curve, key.priv.X, key.priv.Y) //nolint:staticcheck // test-only attacker emulation
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims("user-123", ""))
	tok.Header["kid"] = testKID
	signed, err := tok.SignedString(pubBytes)
	if err != nil {
		t.Fatalf("SignedString(HS256) error = %v", err)
	}
	if _, err := v.Verify(signed); err == nil {
		t.Fatal("Verify() accepted an HS256 token signed with the public key, want error")
	}
}

func TestJWKSVerifyRejectsMissingSubject(t *testing.T) {
	key := newECKey(t, testKID)
	js := newJWKSServer(jwksJSON(t, key))
	defer js.close()
	v := newVerifier(t, js)

	claims := tokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := signES256(t, key, claims)
	if _, err := v.Verify(token); err == nil {
		t.Fatal("Verify() accepted a token without a subject, want error")
	}
}

func TestJWKSVerifyRefetchesOnUnknownKID(t *testing.T) {
	// The first JWKS does NOT contain the signing key's kid; after a refetch the
	// server publishes it, and verification then succeeds. This models a key
	// rotation where the token uses a kid not yet seen by the server.
	key := newECKey(t, testKID)
	other := newECKey(t, "rotated-kid")

	js := newJWKSServer(jwksJSON(t, key)) // initial set: only testKID
	defer js.close()
	v := newVerifier(t, js)

	// Prime the cache with the initial set (verify a token signed by key).
	if _, err := v.Verify(signES256(t, key, validClaims("u", ""))); err != nil {
		t.Fatalf("Verify(initial) error = %v", err)
	}
	hitsAfterPrime := js.hits.Load()

	// Rotate: publish the new key. Use a zero min-refetch interval so the
	// unknown-kid refetch is not rate-limited.
	v.minRefetch = 0
	js.setBody(jwksJSON(t, key, other))

	token := signES256(t, other, validClaims("user-123", ""))
	got, err := v.Verify(token)
	if err != nil {
		t.Fatalf("Verify(rotated) error = %v", err)
	}
	if got.UserID != "user-123" {
		t.Fatalf("Verify() userID = %q, want %q", got.UserID, "user-123")
	}
	if js.hits.Load() <= hitsAfterPrime {
		t.Fatalf("expected a refetch on unknown kid, hits = %d (was %d)", js.hits.Load(), hitsAfterPrime)
	}
}

func TestJWKSVerifyRateLimitsRefetchOnUnknownKID(t *testing.T) {
	// An attacker spamming bogus kids must not force one JWKS fetch per call.
	// After the initial fetch primes the cache, a large min-refetch interval
	// means subsequent unknown-kid verifications do not hit the endpoint again.
	key := newECKey(t, testKID)
	js := newJWKSServer(jwksJSON(t, key))
	defer js.close()
	v := newVerifier(t, js, WithMinRefetchInterval(time.Hour))

	// Prime the cache.
	if _, err := v.Verify(signES256(t, key, validClaims("u", ""))); err != nil {
		t.Fatalf("Verify(initial) error = %v", err)
	}
	hitsAfterPrime := js.hits.Load()

	// Present several tokens with unknown kids; none should trigger a refetch.
	for i := 0; i < 5; i++ {
		bogus := newECKey(t, "bogus-kid")
		if _, err := v.Verify(signES256(t, bogus, validClaims("u", ""))); err == nil {
			t.Fatal("Verify() accepted an unknown-kid token, want error")
		}
	}
	if got := js.hits.Load(); got != hitsAfterPrime {
		t.Fatalf("rate-limit failed: hits = %d, want %d (no refetch)", got, hitsAfterPrime)
	}
}

func TestJWKSVerifyStaleOnErrorForKnownKID(t *testing.T) {
	// Once a kid is cached, a transient JWKS outage must not break auth for that
	// kid: stale-on-error serves the cached key. Use a zero TTL so the cache is
	// considered stale and a refetch (which fails) is attempted on each call.
	key := newECKey(t, testKID)
	js := newJWKSServer(jwksJSON(t, key))
	defer js.close()
	v := newVerifier(t, js, WithMinRefetchInterval(0))

	// Prime the cache with a successful fetch.
	if _, err := v.Verify(signES256(t, key, validClaims("u", ""))); err != nil {
		t.Fatalf("Verify(prime) error = %v", err)
	}

	// Force the cache stale and make the endpoint fail.
	v.ttl = 0
	js.setFailCode(http.StatusInternalServerError)

	// A token with the KNOWN kid must still verify (stale-on-error).
	token := signES256(t, key, validClaims("user-123", ""))
	got, err := v.Verify(token)
	if err != nil {
		t.Fatalf("Verify(stale-on-error, known kid) error = %v, want success", err)
	}
	if got.UserID != "user-123" {
		t.Fatalf("Verify() userID = %q, want %q", got.UserID, "user-123")
	}
}

func TestJWKSVerifyFailClosedWhenJWKSUnreachableAndUncached(t *testing.T) {
	// If the JWKS was never fetched and the endpoint is unreachable, the verifier
	// must reject rather than accept an unverifiable token (fail-closed).
	key := newECKey(t, testKID)
	js := newJWKSServer(jwksJSON(t, key))
	js.setFailCode(http.StatusInternalServerError)
	defer js.close()
	v := newVerifier(t, js)

	token := signES256(t, key, validClaims("user-123", ""))
	if _, err := v.Verify(token); err == nil {
		t.Fatal("Verify() succeeded with unreachable JWKS and empty cache, want error")
	}
}

func TestJWKSVerifyCachesKeySetAcrossCalls(t *testing.T) {
	// Two successive verifications with a known kid and a fresh cache must only
	// fetch the JWKS once.
	key := newECKey(t, testKID)
	js := newJWKSServer(jwksJSON(t, key))
	defer js.close()
	v := newVerifier(t, js)

	for i := 0; i < 3; i++ {
		if _, err := v.Verify(signES256(t, key, validClaims("u", ""))); err != nil {
			t.Fatalf("Verify(%d) error = %v", i, err)
		}
	}
	if got := js.hits.Load(); got != 1 {
		t.Fatalf("JWKS fetched %d times, want 1 (cache hit)", got)
	}
}

func TestJWKSVerifyEnforcesExpectedIssuer(t *testing.T) {
	key := newECKey(t, testKID)
	js := newJWKSServer(jwksJSON(t, key))
	defer js.close()
	v := newVerifier(t, js, WithExpectedIssuer("https://ref.supabase.co/auth/v1"))

	// Wrong issuer -> rejected.
	wrong := tokenClaims{RegisteredClaims: jwt.RegisteredClaims{
		Subject:   "user-123",
		Issuer:    "https://evil.example.com",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
	if _, err := v.Verify(signES256(t, key, wrong)); err == nil {
		t.Fatal("Verify() accepted a token with the wrong issuer, want error")
	}

	// Correct issuer -> accepted.
	good := tokenClaims{RegisteredClaims: jwt.RegisteredClaims{
		Subject:   "user-123",
		Issuer:    "https://ref.supabase.co/auth/v1",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
	if _, err := v.Verify(signES256(t, key, good)); err != nil {
		t.Fatalf("Verify(correct issuer) error = %v", err)
	}
}

func TestNewJWKSVerifierRequiresURL(t *testing.T) {
	if _, err := NewJWKSVerifier(""); err == nil {
		t.Fatal("NewJWKSVerifier(\"\") succeeded, want error")
	}
}
