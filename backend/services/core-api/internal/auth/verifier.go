// Package auth verifies bearer tokens issued by Zitadel and manages the
// mapping between a Zitadel identity and core-api's local User record.
//
// Tenant scoping and role authorization are derived from the local User
// record (looked up by the token's subject), not from Zitadel claims — this
// keeps core-api independent of how a given Zitadel project/organization
// happens to be configured for custom claims.
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

// ZitadelVerifier verifies access tokens issued by a Zitadel instance
// configured to issue JWT (not opaque) access tokens, using the instance's
// OIDC discovery document and JWKS.
type ZitadelVerifier struct {
	verifier *oidc.IDTokenVerifier
}

// NewZitadelVerifier discovers the issuer's OIDC configuration (including
// its JWKS endpoint) and returns a verifier that checks a token's signature,
// issuer, and expiry. audience is the OAuth client ID/API resource the token
// must have been issued for.
//
// discoveryURL and issuerURL are deliberately separate: core-api typically
// reaches Zitadel over an internal address (e.g. a Docker Compose service
// name like "http://zitadel:8080") that isn't the same address Zitadel was
// told is its own public identity (ZITADEL_EXTERNALDOMAIN, e.g.
// "http://localhost:8080" or a LAN IP for testing from a phone). OIDC
// discovery requires the fetched document's "issuer" field to match the URL
// it was fetched from, so a naive single-URL setup breaks the moment those
// two addresses differ. oidc.InsecureIssuerURLContext is the documented
// escape hatch for exactly this split (see its doc comment's Azure
// example); it's a no-op when discoveryURL and issuerURL happen to be equal,
// so this is safe to always apply.
func NewZitadelVerifier(ctx context.Context, discoveryURL, issuerURL, audience string) (*ZitadelVerifier, error) {
	ctx = oidc.InsecureIssuerURLContext(ctx, issuerURL)
	provider, err := oidc.NewProvider(ctx, discoveryURL)
	if err != nil {
		return nil, fmt.Errorf("auth: discover oidc provider at %s: %w", discoveryURL, err)
	}
	return &ZitadelVerifier{verifier: provider.Verifier(&oidc.Config{ClientID: audience})}, nil
}

// Verify checks rawToken's signature, issuer, audience, and expiry, and
// returns its subject.
func (z *ZitadelVerifier) Verify(ctx context.Context, rawToken string) (string, error) {
	idToken, err := z.verifier.Verify(ctx, rawToken)
	if err != nil {
		return "", fmt.Errorf("auth: verify token: %w", err)
	}
	return idToken.Subject, nil
}
