package domain

import "time"

// PricingType is how a product's price is determined at sale time.
type PricingType string

const (
	// PricingPerKg products are weighed and priced by a certified scale —
	// UnitPriceCents is the price per kilogram sent to it.
	PricingPerKg PricingType = "per_kg"
	// PricingPerPiece products are sold by count, never touching a scale —
	// UnitPriceCents is the price per single item, and the line's total is
	// ordinary quantity x price arithmetic (no legal-metrology concern,
	// since no physical measurement is involved).
	PricingPerPiece PricingType = "per_piece"
)

// Product is one item a tenant sells, priced either per kilogram (weighed on
// a scale) or per piece (counted).
type Product struct {
	ID             string      `json:"id"`
	TenantID       string      `json:"tenant_id"`
	Name           string      `json:"name"`
	PricingType    PricingType `json:"pricing_type"`
	UnitPriceCents int         `json:"unit_price_cents"` // per kg if PricingType is PricingPerKg, per piece if PricingPerPiece
	CreatedAt      time.Time   `json:"created_at"`
}
