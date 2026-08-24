package domain

import "time"

// Transaction is one sale-line event: either a scale-approved weigh event
// (what the scale itself measured, priced, and displayed to the customer)
// or a per-piece line (a counted quantity at a fixed price, never touching a
// scale). It is immutable once created — it documents what actually
// happened, so it is never edited, only included in or removed from a draft
// receipt's line items.
type Transaction struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenant_id"`
	UserID      string `json:"user_id"`
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name"` // snapshotted at creation so receipts stay accurate if the product is later renamed or deleted

	PricingType PricingType `json:"pricing_type"`

	// ScaleID and WeightGrams are set only for a PricingPerKg line; empty
	// and zero for a PricingPerPiece line, since no scale was involved.
	ScaleID     string `json:"scale_id"`
	WeightGrams int    `json:"weight_grams"`

	// Quantity is set only for a PricingPerPiece line; zero for a
	// PricingPerKg line.
	Quantity int `json:"quantity"`

	UnitPriceCents  int `json:"unit_price_cents"`  // price per kg (weighed) or per piece (counted) at the time of sale
	TotalPriceCents int `json:"total_price_cents"` // for PricingPerKg, the total the scale computed and displayed; for PricingPerPiece, quantity x unit price

	// ScaleStatusCode is the raw, unverified status field the scale's
	// protocol returned alongside the transaction (see the scale-gateway
	// service's README for why its meaning isn't decoded further). Empty
	// for a PricingPerPiece line.
	ScaleStatusCode string `json:"scale_status_code"`

	CreatedAt time.Time `json:"created_at"`
}
