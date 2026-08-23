package domain

import "time"

// Product is one item a tenant sells, priced per kilogram.
type Product struct {
	ID              string
	TenantID        string
	Name            string
	PricePerKgCents int
	CreatedAt       time.Time
}
