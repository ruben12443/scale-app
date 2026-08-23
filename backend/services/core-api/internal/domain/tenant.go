// Package domain holds the core business types for core-api: tenants
// (market vendors), their users, products, transactions produced by the
// scale, and the draft/finalized receipts built from them. Types here carry
// no storage or transport concerns.
package domain

import "time"

// Tenant is a market vendor business. Each tenant has its own users, product
// catalog, and transaction history.
type Tenant struct {
	ID        string
	Name      string
	CreatedAt time.Time
}
