package domain

import "time"

// Product is one item a tenant sells, priced per kilogram.
type Product struct {
	ID              string    `json:"id"`
	TenantID        string    `json:"tenant_id"`
	Name            string    `json:"name"`
	PricePerKgCents int       `json:"price_per_kg_cents"`
	CreatedAt       time.Time `json:"created_at"`
}
