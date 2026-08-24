package domain

import "time"

// PaymentStatus tracks a payment attempt's lifecycle.
type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusSucceeded PaymentStatus = "succeeded"
	PaymentStatusFailed    PaymentStatus = "failed"
)

// Payment tracks one attempt to collect a finalized receipt's total via the
// mobile app acting as a card-present terminal (Stripe Terminal / Tap to
// Pay). StripePaymentIntentID links it to Stripe's own record.
type Payment struct {
	ID                    string        `json:"id"`
	TenantID              string        `json:"tenant_id"`
	ReceiptID             string        `json:"receipt_id"`
	StripePaymentIntentID string        `json:"stripe_payment_intent_id"`
	AmountCents           int           `json:"amount_cents"`
	Status                PaymentStatus `json:"status"`
	CreatedAt             time.Time     `json:"created_at"`
	UpdatedAt             time.Time     `json:"updated_at"`
}
