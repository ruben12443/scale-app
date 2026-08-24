package payment

import (
	"context"
	"testing"
)

func TestFakeProcessorCreatePaymentIntentAndStatus(t *testing.T) {
	p := NewFakeProcessor()
	ctx := context.Background()

	id, clientSecret, err := p.CreatePaymentIntent(ctx, 624, "chf", map[string]string{"receipt_id": "r1"})
	if err != nil {
		t.Fatalf("CreatePaymentIntent returned error: %v", err)
	}
	if id == "" || clientSecret == "" {
		t.Fatalf("expected non-empty id and client secret, got %q, %q", id, clientSecret)
	}

	status, err := p.GetPaymentIntentStatus(ctx, id)
	if err != nil {
		t.Fatalf("GetPaymentIntentStatus returned error: %v", err)
	}
	if status != "requires_payment_method" {
		t.Fatalf("status = %q, want %q", status, "requires_payment_method")
	}

	p.SetStatus(id, "succeeded")
	status2, err := p.GetPaymentIntentStatus(ctx, id)
	if err != nil {
		t.Fatalf("GetPaymentIntentStatus returned error: %v", err)
	}
	if status2 != "succeeded" {
		t.Fatalf("status = %q, want %q", status2, "succeeded")
	}

	if p.Intents[id].AmountCents != 624 || p.Intents[id].Currency != "chf" || p.Intents[id].Metadata["receipt_id"] != "r1" {
		t.Fatalf("unexpected recorded intent: %+v", p.Intents[id])
	}
}

func TestFakeProcessorGetPaymentIntentStatusUnknown(t *testing.T) {
	p := NewFakeProcessor()
	if _, err := p.GetPaymentIntentStatus(context.Background(), "does-not-exist"); err == nil {
		t.Fatal("expected an error for an unknown payment intent, got nil")
	}
}

func TestFakeProcessorCreateConnectionToken(t *testing.T) {
	p := NewFakeProcessor()
	secret, err := p.CreateConnectionToken(context.Background())
	if err != nil {
		t.Fatalf("CreateConnectionToken returned error: %v", err)
	}
	if secret == "" {
		t.Fatal("expected a non-empty connection token secret")
	}
}
