package auth

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWKS (JSON Web Key Set) verification for Supabase-issued access tokens.
//
// Supabase's current ("active") signing key is asymmetric (ES256, an EC P-256
// key); the legacy HS256 shared secret is only a previously-used fallback. This
// verifier validates tokens against the project's published JWKS, so the server
// never holds any signing secret — it only needs the public keys, which it
// fetches over HTTPS from {SUPABASE_URL}/auth/v1/.well-known/jwks.json (or an
// explicit SUPABASE_JWKS_URL). This is the production-hardening path referenced
// in docs/target-architecture.md §5.
//
// The three design points settled here (documented in docs/deployment.md and
// docs/target-architecture.md):
//
//  1. aud claim: Supabase issues access tokens with aud="authenticated" for
//     signed-in users, but the exact value can vary by project/config. We verify
//     exp + signature + (optionally) iss strictly, but treat aud as TOLERANT by
//     default: a token is not rejected on aud alone unless an expected audience
//     is explicitly configured (WithExpectedAudience). This avoids brittle
//     rejection while keeping the option to tighten. sub is always required.
//
//  2. JWKS unreachable: the last successfully-fetched key set is kept in an
//     in-memory cache with a TTL. On an unknown kid we refetch (rate-limited by
//     a minimum interval, to guard against an attacker spamming bogus kids). If
//     the JWKS endpoint is unreachable but a matching kid is already cached
//     (even past its TTL) we serve it (stale-on-error, for resilience). If no
//     JWKS was ever fetched and the endpoint is unreachable, we reject
//     (fail-closed) — we never accept an unverifiable token.
//
//  3. Algorithms: ES256 (EC P-256) is the primary path. RS256 (RSA n/e) is also
//     supported because parsing it from the JWK is cheap. Any other alg (HS256
//     on an asymmetric key, alg=none, etc.) is rejected.

// jwksDefaultTTL is how long a fetched key set is trusted before a refresh is
// attempted on the next miss. Supabase rotates keys infrequently, so a few
// minutes balances freshness against load on the JWKS endpoint.
const jwksDefaultTTL = 10 * time.Minute

// jwksMinRefetchInterval rate-limits refetches triggered by unknown kids. Without
// it, a client presenting tokens with random kids could force one HTTP request
// per call (a JWKS-endpoint amplification / DoS vector).
const jwksMinRefetchInterval = 30 * time.Second

// jwksHTTPTimeout bounds each JWKS fetch so a hung network never blocks token
// verification (and therefore every authenticated RPC) indefinitely.
const jwksHTTPTimeout = 10 * time.Second

// jwksAllowedAlgs are the asymmetric signing algorithms we accept. HS256 and
// "none" are deliberately excluded: an HS256 token verified against a public key
// is the classic algorithm-confusion attack, and "none" is unsigned.
var jwksAllowedAlgs = []string{"ES256", "RS256"}

// httpDoer is the minimal HTTP client surface the verifier needs, so tests can
// inject an httptest.Server-backed client.
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// jwk is a single JSON Web Key as published by Supabase/GoTrue. Only the fields
// needed to reconstruct EC P-256 and RSA public keys are decoded.
type jwk struct {
	Kty string `json:"kty"`           // key type: "EC" or "RSA"
	Kid string `json:"kid"`           // key id, matched against the token header
	Alg string `json:"alg"`           // e.g. "ES256", "RS256"
	Use string `json:"use,omitempty"` // "sig" expected
	Crv string `json:"crv,omitempty"` // EC curve, e.g. "P-256"
	X   string `json:"x,omitempty"`   // EC x (base64url)
	Y   string `json:"y,omitempty"`   // EC y (base64url)
	N   string `json:"n,omitempty"`   // RSA modulus (base64url)
	E   string `json:"e,omitempty"`   // RSA public exponent (base64url)
}

// jwksDocument is the top-level JWKS JSON ({"keys":[...]}).
type jwksDocument struct {
	Keys []jwk `json:"keys"`
}

// JWKSVerifier validates asymmetric (ES256/RS256) JWTs against a project's
// published JWKS. It implements Verifier and returns the same Identity as the
// HMAC path (UserID = "sub", Email = "email" claim).
type JWKSVerifier struct {
	jwksURL string
	client  httpDoer

	ttl              time.Duration
	minRefetch       time.Duration
	expectedIssuer   string // verified strictly when non-empty
	expectedAudience string // verified strictly when non-empty (tolerant otherwise)

	mu          sync.Mutex
	keys        map[string]any // kid -> *ecdsa.PublicKey | *rsa.PublicKey
	fetchedAt   time.Time      // zero until the first successful fetch
	lastAttempt time.Time      // last fetch attempt (success or failure)
}

// JWKSOption configures a JWKSVerifier.
type JWKSOption func(*JWKSVerifier)

// WithHTTPClient injects a custom HTTP client (used by tests).
func WithHTTPClient(c httpDoer) JWKSOption {
	return func(v *JWKSVerifier) { v.client = c }
}

// WithCacheTTL overrides how long a fetched key set is trusted before a refresh
// is attempted on the next cache miss.
func WithCacheTTL(ttl time.Duration) JWKSOption {
	return func(v *JWKSVerifier) {
		if ttl > 0 {
			v.ttl = ttl
		}
	}
}

// WithMinRefetchInterval overrides the anti-abuse minimum interval between
// refetches triggered by unknown kids.
func WithMinRefetchInterval(d time.Duration) JWKSOption {
	return func(v *JWKSVerifier) { v.minRefetch = d }
}

// WithExpectedIssuer enables strict issuer (iss) verification.
func WithExpectedIssuer(iss string) JWKSOption {
	return func(v *JWKSVerifier) { v.expectedIssuer = iss }
}

// WithExpectedAudience enables strict audience (aud) verification. When unset,
// the aud claim is tolerated (not rejected) — see the package design notes.
func WithExpectedAudience(aud string) JWKSOption {
	return func(v *JWKSVerifier) { v.expectedAudience = aud }
}

// NewJWKSVerifier returns a Verifier that validates ES256/RS256 tokens against
// the JWKS published at jwksURL. The key set is fetched lazily on the first
// Verify and cached with a TTL.
func NewJWKSVerifier(jwksURL string, opts ...JWKSOption) (*JWKSVerifier, error) {
	if jwksURL == "" {
		return nil, errors.New("auth: JWKS URL is required")
	}
	v := &JWKSVerifier{
		jwksURL:    jwksURL,
		client:     &http.Client{Timeout: jwksHTTPTimeout},
		ttl:        jwksDefaultTTL,
		minRefetch: jwksMinRefetchInterval,
		keys:       map[string]any{},
	}
	for _, opt := range opts {
		opt(v)
	}
	return v, nil
}

// Verify implements Verifier. It resolves the token's signing key by kid from
// the cached JWKS (refetching when needed), enforces the allowed asymmetric
// algorithms, validates the signature, expiry and optional iss/aud, then returns
// the subject and optional email as an Identity.
func (v *JWKSVerifier) Verify(token string) (Identity, error) {
	var claims tokenClaims

	parserOpts := []jwt.ParserOption{jwt.WithValidMethods(jwksAllowedAlgs)}
	if v.expectedIssuer != "" {
		parserOpts = append(parserOpts, jwt.WithIssuer(v.expectedIssuer))
	}
	if v.expectedAudience != "" {
		parserOpts = append(parserOpts, jwt.WithAudience(v.expectedAudience))
	}

	_, err := jwt.ParseWithClaims(token, &claims, v.keyfunc, parserOpts...)
	if err != nil {
		return Identity{}, fmt.Errorf("auth: invalid token: %w", err)
	}

	if claims.Subject == "" {
		return Identity{}, ErrNoSubject
	}
	return Identity{UserID: claims.Subject, Email: claims.Email}, nil
}

// keyfunc is the jwt.Keyfunc resolving the public key for the token's kid. It is
// where the cache/refetch/fail-closed policy lives.
func (v *JWKSVerifier) keyfunc(t *jwt.Token) (any, error) {
	// Reject the algorithm-confusion / unsigned cases before touching any key:
	// only the EC and RSA families are acceptable here.
	switch t.Method.(type) {
	case *jwt.SigningMethodECDSA, *jwt.SigningMethodRSA:
	default:
		return nil, fmt.Errorf("auth: unexpected signing method %q", t.Header["alg"])
	}

	kid, _ := t.Header["kid"].(string)
	if kid == "" {
		return nil, errors.New("auth: token header has no kid")
	}

	key, err := v.keyForKid(kid)
	if err != nil {
		return nil, err
	}
	return key, nil
}

// keyForKid returns the cached public key for kid, refreshing the JWKS when the
// kid is unknown or the cache has expired. It implements the unreachable-JWKS
// policy: stale-on-error when the kid is already cached, fail-closed otherwise.
func (v *JWKSVerifier) keyForKid(kid string) (any, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now()
	cacheFresh := !v.fetchedAt.IsZero() && now.Sub(v.fetchedAt) < v.ttl

	if key, ok := v.keys[kid]; ok && cacheFresh {
		return key, nil
	}

	// Either the kid is unknown or the cache is stale: attempt a refetch, but
	// rate-limit refetches triggered while we still have *some* cached keys so a
	// flood of bogus kids cannot hammer the JWKS endpoint. The very first fetch
	// (no keys yet) is never rate-limited, so startup is not delayed.
	mayFetch := v.fetchedAt.IsZero() || now.Sub(v.lastAttempt) >= v.minRefetch
	if mayFetch {
		v.lastAttempt = now
		keys, err := v.fetch()
		if err == nil {
			v.keys = keys
			v.fetchedAt = now
			if key, ok := keys[kid]; ok {
				return key, nil
			}
			return nil, fmt.Errorf("auth: no key for kid %q in JWKS", kid)
		}
		// Fetch failed. Fall back to whatever we still have cached.
		if key, ok := v.keys[kid]; ok {
			// stale-on-error: a previously valid key for this exact kid keeps
			// authentication working through a transient JWKS outage.
			return key, nil
		}
		// fail-closed: nothing usable cached for this kid -> reject.
		return nil, fmt.Errorf("auth: JWKS fetch failed and kid %q not cached: %w", kid, err)
	}

	// Refetch was rate-limited. Serve the cached key for this kid if we have one
	// (it is past TTL but still the project's published key); otherwise reject
	// rather than spam the endpoint.
	if key, ok := v.keys[kid]; ok {
		return key, nil
	}
	return nil, fmt.Errorf("auth: no key for kid %q (refetch rate-limited)", kid)
}

// fetch retrieves and parses the JWKS, returning a kid -> public key map. It
// never mutates verifier state; the caller owns the lock and the cache update.
func (v *JWKSVerifier) fetch() (map[string]any, error) {
	req, err := http.NewRequest(http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return nil, fmt.Errorf("auth: building JWKS request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth: fetching JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth: JWKS endpoint returned status %d", resp.StatusCode)
	}

	var doc jwksDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("auth: decoding JWKS: %w", err)
	}

	keys := make(map[string]any, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kid == "" {
			continue // a key without a kid cannot be matched to a token header
		}
		pub, err := parseJWK(k)
		if err != nil {
			// Skip keys we can't parse (e.g. an unsupported kty) rather than
			// failing the whole set — other keys may still be usable.
			continue
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return nil, errors.New("auth: JWKS contained no usable keys")
	}
	return keys, nil
}

// parseJWK converts a single JWK into a crypto public key (*ecdsa.PublicKey or
// *rsa.PublicKey). Only EC P-256 and RSA are supported.
func parseJWK(k jwk) (any, error) {
	switch k.Kty {
	case "EC":
		return parseECJWK(k)
	case "RSA":
		return parseRSAJWK(k)
	default:
		return nil, fmt.Errorf("auth: unsupported JWK kty %q", k.Kty)
	}
}

// parseECJWK reconstructs an ECDSA public key from an EC JWK. Only the P-256
// curve (Supabase's ES256 key) is supported.
func parseECJWK(k jwk) (*ecdsa.PublicKey, error) {
	if k.Crv != "P-256" {
		return nil, fmt.Errorf("auth: unsupported EC curve %q", k.Crv)
	}
	x, err := b64uintBytes(k.X)
	if err != nil {
		return nil, fmt.Errorf("auth: decoding EC x: %w", err)
	}
	y, err := b64uintBytes(k.Y)
	if err != nil {
		return nil, fmt.Errorf("auth: decoding EC y: %w", err)
	}
	// Validate that (x, y) is a real point on P-256 using crypto/ecdh, whose
	// NewPublicKey performs the on-curve check (and rejects the identity). This
	// replaces the deprecated elliptic.Curve.IsOnCurve. The coordinates are
	// left-padded to the curve's 32-byte field size and packed into the
	// uncompressed SEC1 encoding (0x04 || X || Y) that ecdh expects.
	const p256CoordLen = 32 // P-256 field elements are 32 bytes
	if len(x) > p256CoordLen || len(y) > p256CoordLen {
		return nil, errors.New("auth: EC coordinate exceeds P-256 field size")
	}
	sec1 := make([]byte, 1+2*p256CoordLen)
	sec1[0] = 4 // uncompressed point marker
	copy(sec1[1+p256CoordLen-len(x):1+p256CoordLen], x)
	copy(sec1[1+2*p256CoordLen-len(y):], y)
	if _, err := ecdh.P256().NewPublicKey(sec1); err != nil {
		return nil, fmt.Errorf("auth: EC public key is not a valid P-256 point: %w", err)
	}
	return &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(x),
		Y:     new(big.Int).SetBytes(y),
	}, nil
}

// parseRSAJWK reconstructs an RSA public key from an RSA JWK (modulus n,
// exponent e). RS256 support is cheap, so it is included.
func parseRSAJWK(k jwk) (*rsa.PublicKey, error) {
	nBytes, err := b64uintBytes(k.N)
	if err != nil {
		return nil, fmt.Errorf("auth: decoding RSA n: %w", err)
	}
	eBytes, err := b64uintBytes(k.E)
	if err != nil {
		return nil, fmt.Errorf("auth: decoding RSA e: %w", err)
	}
	if len(nBytes) == 0 || len(eBytes) == 0 {
		return nil, errors.New("auth: RSA JWK missing n or e")
	}
	// e is a big-endian unsigned integer. A real RSA exponent fits in a few bytes
	// (65537 is by far the most common); reject anything that cannot fit in the
	// 8-byte window before padding, both to bound it to a valid int and to avoid
	// a negative slice index / panic on a malformed JWK.
	if len(eBytes) > 8 {
		return nil, fmt.Errorf("auth: RSA public exponent too large (%d bytes)", len(eBytes))
	}
	// Left-pad to 8 bytes for binary decode.
	eFull := make([]byte, 8)
	copy(eFull[8-len(eBytes):], eBytes)
	e := binary.BigEndian.Uint64(eFull)
	if e == 0 {
		return nil, errors.New("auth: RSA public exponent is zero")
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(e),
	}, nil
}

// b64uintBytes decodes a base64url (no padding) JWK field into its raw bytes.
func b64uintBytes(s string) ([]byte, error) {
	if s == "" {
		return nil, errors.New("empty field")
	}
	return base64.RawURLEncoding.DecodeString(s)
}
