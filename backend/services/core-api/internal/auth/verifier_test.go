package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
)

// fakeOIDCProvider is a minimal OIDC discovery + JWKS server backed by a
// real RSA key pair, so RauthyVerifier's actual signature/issuer/audience/
// expiry verification logic is exercised end to end, not just its plumbing.
type fakeOIDCProvider struct {
	server     *httptest.Server
	privateKey *rsa.PrivateKey
	keyID      string

	// advertisedIssuer is what the discovery document's "issuer" field
	// reports. It defaults to the server's own URL; tests exercising the
	// discoveryURL != issuerURL split set it to a different value to mimic
	// Rauthy advertising its externally-configured issuer even though it
	// was reached at an internal address.
	advertisedIssuer string
}

func newFakeOIDCProvider(t *testing.T) *fakeOIDCProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	p := &fakeOIDCProvider{privateKey: key, keyID: "test-key-1"}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", p.serveDiscovery)
	mux.HandleFunc("/keys", p.serveJWKS)
	p.server = httptest.NewServer(mux)
	t.Cleanup(p.server.Close)
	p.advertisedIssuer = p.server.URL
	return p
}

func (p *fakeOIDCProvider) serveDiscovery(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"issuer":                 p.advertisedIssuer,
		"jwks_uri":               p.server.URL + "/keys",
		"authorization_endpoint": p.server.URL + "/authorize",
		"token_endpoint":         p.server.URL + "/token",
	})
}

func (p *fakeOIDCProvider) serveJWKS(w http.ResponseWriter, r *http.Request) {
	set := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       &p.privateKey.PublicKey,
		KeyID:     p.keyID,
		Algorithm: "RS256",
		Use:       "sig",
	}}}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(set)
}

type testClaims struct {
	Issuer   string `json:"iss"`
	Subject  string `json:"sub"`
	Audience string `json:"aud"`
	Expiry   int64  `json:"exp"`
	IssuedAt int64  `json:"iat"`
}

// sign builds and signs a JWT with the given claims using the provider's
// private key (or a mismatched throwaway key, for negative tests).
func (p *fakeOIDCProvider) sign(t *testing.T, claims testClaims, useWrongKey bool) string {
	t.Helper()
	signingKey := p.privateKey
	if useWrongKey {
		var err error
		signingKey, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("generate mismatched RSA key: %v", err)
		}
	}

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: signingKey},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", p.keyID),
	)
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}

	jws, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	compact, err := jws.CompactSerialize()
	if err != nil {
		t.Fatalf("serialize token: %v", err)
	}
	return compact
}

const testAudience = "core-api"

func TestRauthyVerifierAcceptsValidToken(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	ctx := t.Context()

	verifier, err := NewRauthyVerifier(ctx, provider.server.URL, provider.server.URL, testAudience)
	if err != nil {
		t.Fatalf("NewRauthyVerifier returned error: %v", err)
	}

	token := provider.sign(t, testClaims{
		Issuer:   provider.server.URL,
		Subject:  "rauthy-subject-123",
		Audience: testAudience,
		Expiry:   time.Now().Add(time.Hour).Unix(),
		IssuedAt: time.Now().Unix(),
	}, false)

	subject, err := verifier.Verify(ctx, token)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if subject != "rauthy-subject-123" {
		t.Fatalf("subject = %q, want %q", subject, "rauthy-subject-123")
	}
}

func TestRauthyVerifierRejectsExpiredToken(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	ctx := t.Context()

	verifier, err := NewRauthyVerifier(ctx, provider.server.URL, provider.server.URL, testAudience)
	if err != nil {
		t.Fatalf("NewRauthyVerifier returned error: %v", err)
	}

	token := provider.sign(t, testClaims{
		Issuer:   provider.server.URL,
		Subject:  "rauthy-subject-123",
		Audience: testAudience,
		Expiry:   time.Now().Add(-time.Hour).Unix(),
		IssuedAt: time.Now().Add(-2 * time.Hour).Unix(),
	}, false)

	if _, err := verifier.Verify(ctx, token); err == nil {
		t.Fatal("expected an error for an expired token, got nil")
	}
}

func TestRauthyVerifierRejectsWrongAudience(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	ctx := t.Context()

	verifier, err := NewRauthyVerifier(ctx, provider.server.URL, provider.server.URL, testAudience)
	if err != nil {
		t.Fatalf("NewRauthyVerifier returned error: %v", err)
	}

	token := provider.sign(t, testClaims{
		Issuer:   provider.server.URL,
		Subject:  "rauthy-subject-123",
		Audience: "some-other-api",
		Expiry:   time.Now().Add(time.Hour).Unix(),
		IssuedAt: time.Now().Unix(),
	}, false)

	if _, err := verifier.Verify(ctx, token); err == nil {
		t.Fatal("expected an error for a token issued for a different audience, got nil")
	}
}

func TestRauthyVerifierRejectsWrongSigningKey(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	ctx := t.Context()

	verifier, err := NewRauthyVerifier(ctx, provider.server.URL, provider.server.URL, testAudience)
	if err != nil {
		t.Fatalf("NewRauthyVerifier returned error: %v", err)
	}

	token := provider.sign(t, testClaims{
		Issuer:   provider.server.URL,
		Subject:  "rauthy-subject-123",
		Audience: testAudience,
		Expiry:   time.Now().Add(time.Hour).Unix(),
		IssuedAt: time.Now().Unix(),
	}, true) // signed with a key never published in the JWKS

	if _, err := verifier.Verify(ctx, token); err == nil {
		t.Fatal("expected an error for a token signed with an unknown key, got nil")
	}
}

// TestRauthyVerifierSupportsDiscoveryIssuerSplit proves the actual reason
// NewRauthyVerifier takes separate discoveryURL and issuerURL arguments:
// core-api reaches Rauthy over one address (e.g. an internal Docker Compose
// hostname) while Rauthy's discovery document advertises a different
// externally-configured issuer (e.g. localhost or a LAN IP). Without
// oidc.InsecureIssuerURLContext, discovery would fail because the fetched
// document's "issuer" field wouldn't match the URL it was fetched from.
func TestRauthyVerifierSupportsDiscoveryIssuerSplit(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	const externalIssuer = "http://external.example.com"
	provider.advertisedIssuer = externalIssuer
	ctx := t.Context()

	verifier, err := NewRauthyVerifier(ctx, provider.server.URL, externalIssuer, testAudience)
	if err != nil {
		t.Fatalf("NewRauthyVerifier returned error: %v", err)
	}

	token := provider.sign(t, testClaims{
		Issuer:   externalIssuer,
		Subject:  "rauthy-subject-123",
		Audience: testAudience,
		Expiry:   time.Now().Add(time.Hour).Unix(),
		IssuedAt: time.Now().Unix(),
	}, false)

	subject, err := verifier.Verify(ctx, token)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if subject != "rauthy-subject-123" {
		t.Fatalf("subject = %q, want %q", subject, "rauthy-subject-123")
	}
}

// TestRauthyVerifierRejectsMismatchedTokenIssuer confirms the split doesn't
// disable issuer checking entirely: a token claiming an issuer other than
// the configured issuerURL must still be rejected.
func TestRauthyVerifierRejectsMismatchedTokenIssuer(t *testing.T) {
	provider := newFakeOIDCProvider(t)
	const externalIssuer = "http://external.example.com"
	provider.advertisedIssuer = externalIssuer
	ctx := t.Context()

	verifier, err := NewRauthyVerifier(ctx, provider.server.URL, externalIssuer, testAudience)
	if err != nil {
		t.Fatalf("NewRauthyVerifier returned error: %v", err)
	}

	token := provider.sign(t, testClaims{
		Issuer:   "http://attacker.example.com",
		Subject:  "rauthy-subject-123",
		Audience: testAudience,
		Expiry:   time.Now().Add(time.Hour).Unix(),
		IssuedAt: time.Now().Unix(),
	}, false)

	if _, err := verifier.Verify(ctx, token); err == nil {
		t.Fatal("expected an error for a token with a mismatched issuer, got nil")
	}
}
