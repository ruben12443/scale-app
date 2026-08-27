// Package auth verifies bearer tokens issued by Rauthy and manages the
// mapping between a Rauthy identity and core-api's local User record.
//
// Tenant scoping and role authorization are derived from the local User
// record (looked up by the token's subject), not from Rauthy claims — this
// keeps core-api independent of how a given Rauthy instance happens to be
// configured for custom claims (Rauthy, unlike Zitadel, has no
// project/organization concept of its own in the first place).
package auth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
)

// TokenVerifier verifies a raw bearer token and returns the subject it was
// issued for.
type TokenVerifier interface {
	Verify(ctx context.Context, rawToken string) (subject string, err error)
}

// RauthyVerifier verifies access tokens issued by a Rauthy instance
// configured to issue JWT (not opaque) access tokens, using the instance's
// OIDC discovery document and JWKS.
type RauthyVerifier struct {
	verifier *oidc.IDTokenVerifier
}

// NewRauthyVerifier discovers the issuer's OIDC configuration (including
// its JWKS endpoint) and returns a verifier that checks a token's signature,
// issuer, and expiry. audience is the OAuth client ID/API resource the token
// must have been issued for.
//
// discoveryURL and issuerURL are deliberately separate: core-api typically
// reaches Rauthy over an internal address (e.g. a Docker Compose service
// name like "http://rauthy:8080") that isn't the same address Rauthy was
// told is its own public identity (server.pub_url / PUB_URL, e.g.
// "http://localhost:8080" or a LAN IP for testing from a phone). OIDC
// discovery requires the fetched document's "issuer" field to match the URL
// it was fetched from, so a naive single-URL setup breaks the moment those
// two addresses differ. oidc.InsecureIssuerURLContext is the documented
// escape hatch for exactly this split (see its doc comment's Azure
// example); it's a no-op when discoveryURL and issuerURL happen to be equal,
// so this is safe to always apply.
func NewRauthyVerifier(ctx context.Context, discoveryURL, issuerURL, audience string) (*RauthyVerifier, error) {
	ctx = oidc.InsecureIssuerURLContext(ctx, issuerURL)
	provider, err := oidc.NewProvider(ctx, discoveryURL)
	if err != nil {
		return nil, fmt.Errorf("auth: discover oidc provider at %s: %w", discoveryURL, err)
	}
	return &RauthyVerifier{verifier: provider.Verifier(&oidc.Config{ClientID: audience})}, nil
}

// Verify checks rawToken's signature, issuer, audience, and expiry, and
// returns its subject.
func (r *RauthyVerifier) Verify(ctx context.Context, rawToken string) (string, error) {
	idToken, err := r.verifier.Verify(ctx, rawToken)
	if err != nil {
		return "", fmt.Errorf("auth: verify token: %w", err)
	}
	return idToken.Subject, nil
}
