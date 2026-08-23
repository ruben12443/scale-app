package domain

import "time"

// Transaction is one scale-approved weigh event: the record of what the
// scale itself measured, priced, and displayed to the customer. It is
// immutable once created — it documents what actually happened on the
// scale, so it is never edited, only included in or removed from a draft
// receipt's line items.
type Transaction struct {
	ID        string
	TenantID  string
	UserID    string
	ProductID string
	ScaleID   string

	WeightGrams     int
	UnitPriceCents  int // price per kg that was sent to the scale
	TotalPriceCents int // total price the scale computed and displayed

	// ScaleStatusCode is the raw, unverified status field the scale's
	// protocol returned alongside the transaction (see the scale-gateway
	// service's README for why its meaning isn't decoded further).
	ScaleStatusCode string

	CreatedAt time.Time
}
