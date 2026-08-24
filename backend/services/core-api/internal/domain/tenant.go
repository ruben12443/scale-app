// Package domain holds the core business types for core-api: tenants
// (market vendors), their users, products, transactions produced by the
// scale, and the draft/finalized receipts built from them. Types here carry
// no storage or transport concerns.
package domain

import "time"

// Tenant is a market vendor business. Each tenant has its own users, product
// catalog, and transaction history.
//
// ID doubles as the corresponding Zitadel organization ID: a tenant here is
// exactly one Zitadel org, so creating a vendor user only needs an org ID to
// scope it to, with no separate mapping table.
type Tenant struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}
