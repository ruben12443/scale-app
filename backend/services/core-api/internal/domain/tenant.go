// Package domain holds the core business types for core-api: tenants
// (market vendors), their users, products, transactions produced by the
// scale, and the draft/finalized receipts built from them. Types here carry
// no storage or transport concerns.
package domain

import "time"

// Tenant is a market vendor business. Each tenant has its own users, product
// catalog, and transaction history.
//
// Tenant scoping is purely local to core-api's own database — Rauthy (unlike
// Zitadel, which this replaced) has no organization/multi-tenancy concept of
// its own, so it just issues one flat set of user identities and core-api is
// solely responsible for which tenant a given RauthySubjectID belongs to,
// via the User record that links them.
type Tenant struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}
