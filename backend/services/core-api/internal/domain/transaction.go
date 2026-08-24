package domain

import "time"

// Transaction is one scale-approved weigh event: the record of what the
// scale itself measured, priced, and displayed to the customer. It is
// immutable once created — it documents what actually happened on the
// scale, so it is never edited, only included in or removed from a draft
// receipt's line items.
type Transaction struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenant_id"`
	UserID      string `json:"user_id"`
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name"` // snapshotted at creation so receipts stay accurate if the product is later renamed or deleted
	ScaleID     string `json:"scale_id"`

	WeightGrams     int `json:"weight_grams"`
	UnitPriceCents  int `json:"unit_price_cents"`  // price per kg that was sent to the scale
	TotalPriceCents int `json:"total_price_cents"` // total price the scale computed and displayed

	// ScaleStatusCode is the raw, unverified status field the scale's
	// protocol returned alongside the transaction (see the scale-gateway
	// service's README for why its meaning isn't decoded further).
	ScaleStatusCode string `json:"scale_status_code"`

	CreatedAt time.Time `json:"created_at"`
}
