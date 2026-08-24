// Package payment integrates with Stripe for phone-as-terminal (Stripe
// Terminal / Tap to Pay) card-present payments: a connection token lets the
// mobile app's Terminal SDK connect, and a PaymentIntent represents one
// charge attempt against a finalized receipt.
package payment

import (
	"context"
	"fmt"

	"github.com/stripe/stripe-go/v82"
)

// Processor is the payment operations core-api needs, independent of
// Stripe's own types so it's easy to fake in tests.
type Processor interface {
	// CreateConnectionToken returns a short-lived secret the mobile app's
	// Stripe Terminal SDK uses to connect to a card reader / Tap to Pay.
	CreateConnectionToken(ctx context.Context) (secret string, err error)
	// CreatePaymentIntent starts a card-present charge for a finalized
	// receipt. metadata should include enough to trace the charge back to
	// the receipt (e.g. a "receipt_id" entry).
	CreatePaymentIntent(ctx context.Context, amountCents int64, currency string, metadata map[string]string) (paymentIntentID, clientSecret string, err error)
	// GetPaymentIntentStatus retrieves a payment intent's current status
	// directly from Stripe (a fallback to polling if a webhook is missed).
	GetPaymentIntentStatus(ctx context.Context, paymentIntentID string) (status string, err error)
}

// StripeProcessor implements Processor against the real Stripe API.
type StripeProcessor struct {
	client *stripe.Client
}

// NewStripeProcessor builds a StripeProcessor authenticated with secretKey
// (a Stripe secret API key).
func NewStripeProcessor(secretKey string) *StripeProcessor {
	return &StripeProcessor{client: stripe.NewClient(secretKey)}
}

func (p *StripeProcessor) CreateConnectionToken(ctx context.Context) (string, error) {
	token, err := p.client.V1TerminalConnectionTokens.Create(ctx, &stripe.TerminalConnectionTokenCreateParams{})
	if err != nil {
		return "", fmt.Errorf("payment: create connection token: %w", err)
	}
	return token.Secret, nil
}

func (p *StripeProcessor) CreatePaymentIntent(ctx context.Context, amountCents int64, currency string, metadata map[string]string) (string, string, error) {
	params := &stripe.PaymentIntentCreateParams{
		Amount:             stripe.Int64(amountCents),
		Currency:           stripe.String(currency),
		PaymentMethodTypes: []*string{stripe.String("card_present")},
		CaptureMethod:      stripe.String("automatic"),
		Metadata:           metadata,
	}
	intent, err := p.client.V1PaymentIntents.Create(ctx, params)
	if err != nil {
		return "", "", fmt.Errorf("payment: create payment intent: %w", err)
	}
	return intent.ID, intent.ClientSecret, nil
}

func (p *StripeProcessor) GetPaymentIntentStatus(ctx context.Context, paymentIntentID string) (string, error) {
	intent, err := p.client.V1PaymentIntents.Retrieve(ctx, paymentIntentID, nil)
	if err != nil {
		return "", fmt.Errorf("payment: get payment intent: %w", err)
	}
	return string(intent.Status), nil
}
