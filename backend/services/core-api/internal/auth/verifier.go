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
func NewZitadelVerifier(ctx context.Context, issuerURL, audience string) (*ZitadelVerifier, error) {
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("auth: discover oidc provider at %s: %w", issuerURL, err)
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
