package payment

import (
	"context"
	"fmt"
	"sync"
)

// FakeIntent records one call to FakeProcessor.CreatePaymentIntent, plus
// whatever status a test has since set on it.
type FakeIntent struct {
	AmountCents int64
	Currency    string
	Metadata    map[string]string
	Status      string
}

// FakeProcessor is an in-memory Processor for tests, with no dependency on
// a live Stripe account.
type FakeProcessor struct {
	mu      sync.Mutex
	nextID  int
	Intents map[string]FakeIntent
}

// NewFakeProcessor returns an empty FakeProcessor.
func NewFakeProcessor() *FakeProcessor {
	return &FakeProcessor{Intents: map[string]FakeIntent{}}
}

func (f *FakeProcessor) CreateConnectionToken(ctx context.Context) (string, error) {
	return "fake-connection-token-secret", nil
}

func (f *FakeProcessor) CreatePaymentIntent(ctx context.Context, amountCents int64, currency string, metadata map[string]string) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	id := fmt.Sprintf("pi_fake_%d", f.nextID)
	f.Intents[id] = FakeIntent{AmountCents: amountCents, Currency: currency, Metadata: metadata, Status: "requires_payment_method"}
	return id, "cs_fake_" + id, nil
}

func (f *FakeProcessor) GetPaymentIntentStatus(ctx context.Context, paymentIntentID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	intent, ok := f.Intents[paymentIntentID]
	if !ok {
		return "", fmt.Errorf("payment: fake processor: unknown payment intent %q", paymentIntentID)
	}
	return intent.Status, nil
}

// SetStatus lets a test simulate Stripe reporting a status change (e.g. as
// if a webhook had been received).
func (f *FakeProcessor) SetStatus(paymentIntentID, status string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	intent := f.Intents[paymentIntentID]
	intent.Status = status
	f.Intents[paymentIntentID] = intent
}
