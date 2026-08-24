package domain

import (
	"errors"
	"testing"
	"time"
)

func newDraftReceipt() *Receipt {
	return &Receipt{ID: "r1", TenantID: "t1", UserID: "u1", Status: ReceiptStatusDraft, CreatedAt: time.Now()}
}

func TestReceiptAddLine(t *testing.T) {
	r := newDraftReceipt()
	if err := r.AddLine("tx-1"); err != nil {
		t.Fatalf("AddLine returned error: %v", err)
	}
	if err := r.AddLine("tx-2"); err != nil {
		t.Fatalf("AddLine returned error: %v", err)
	}
	if len(r.TransactionIDs) != 2 || r.TransactionIDs[0] != "tx-1" || r.TransactionIDs[1] != "tx-2" {
		t.Fatalf("TransactionIDs = %v, want [tx-1 tx-2]", r.TransactionIDs)
	}
}

func TestReceiptRemoveLine(t *testing.T) {
	r := newDraftReceipt()
	_ = r.AddLine("tx-1")
	_ = r.AddLine("tx-2")
	_ = r.AddLine("tx-3")

	removed, err := r.RemoveLine("tx-2")
	if err != nil {
		t.Fatalf("RemoveLine returned error: %v", err)
	}
	if !removed {
		t.Fatal("RemoveLine reported not found for a line that exists")
	}
	if len(r.TransactionIDs) != 2 || r.TransactionIDs[0] != "tx-1" || r.TransactionIDs[1] != "tx-3" {
		t.Fatalf("TransactionIDs = %v, want [tx-1 tx-3]", r.TransactionIDs)
	}
}

func TestReceiptRemoveLineNotFound(t *testing.T) {
	r := newDraftReceipt()
	_ = r.AddLine("tx-1")

	removed, err := r.RemoveLine("does-not-exist")
	if err != nil {
		t.Fatalf("RemoveLine returned error: %v", err)
	}
	if removed {
		t.Fatal("RemoveLine reported found for a line that doesn't exist")
	}
	if len(r.TransactionIDs) != 1 {
		t.Fatalf("TransactionIDs = %v, want unchanged [tx-1]", r.TransactionIDs)
	}
}

func TestReceiptFinalize(t *testing.T) {
	r := newDraftReceipt()
	_ = r.AddLine("tx-1")

	now := time.Now()
	if err := r.Finalize(42, now); err != nil {
		t.Fatalf("Finalize returned error: %v", err)
	}
	if r.Status != ReceiptStatusFinalized {
		t.Fatalf("Status = %q, want %q", r.Status, ReceiptStatusFinalized)
	}
	if r.Number != 42 {
		t.Fatalf("Number = %d, want 42", r.Number)
	}
	if r.FinalizedAt == nil || !r.FinalizedAt.Equal(now) {
		t.Fatalf("FinalizedAt = %v, want %v", r.FinalizedAt, now)
	}
}

func TestFinalizedReceiptRejectsMutation(t *testing.T) {
	r := newDraftReceipt()
	_ = r.AddLine("tx-1")
	_ = r.Finalize(1, time.Now())

	if err := r.AddLine("tx-2"); !errors.Is(err, ErrReceiptFinalized) {
		t.Fatalf("AddLine on finalized receipt returned %v, want %v", err, ErrReceiptFinalized)
	}
	if _, err := r.RemoveLine("tx-1"); !errors.Is(err, ErrReceiptFinalized) {
		t.Fatalf("RemoveLine on finalized receipt returned %v, want %v", err, ErrReceiptFinalized)
	}
	if err := r.Finalize(2, time.Now()); !errors.Is(err, ErrReceiptFinalized) {
		t.Fatalf("double Finalize returned %v, want %v", err, ErrReceiptFinalized)
	}
	// Finalizing twice must not clobber the original number/timestamp.
	if r.Number != 1 {
		t.Fatalf("Number = %d, want unchanged 1", r.Number)
	}
}

func TestReceiptReopen(t *testing.T) {
	r := newDraftReceipt()
	_ = r.AddLine("tx-1")
	_ = r.Finalize(7, time.Now())

	if err := r.Reopen(); err != nil {
		t.Fatalf("Reopen returned error: %v", err)
	}
	if r.Status != ReceiptStatusDraft {
		t.Fatalf("Status = %q, want %q", r.Status, ReceiptStatusDraft)
	}
	if r.Number != 0 {
		t.Fatalf("Number = %d, want 0 after reopen", r.Number)
	}
	if r.FinalizedAt != nil {
		t.Fatalf("FinalizedAt = %v, want nil after reopen", r.FinalizedAt)
	}

	// A reopened draft accepts edits again.
	if err := r.AddLine("tx-2"); err != nil {
		t.Fatalf("AddLine after reopen returned error: %v", err)
	}
}

func TestReceiptReopenRejectsDraft(t *testing.T) {
	r := newDraftReceipt()
	if err := r.Reopen(); !errors.Is(err, ErrReceiptNotFinalized) {
		t.Fatalf("Reopen on draft returned %v, want %v", err, ErrReceiptNotFinalized)
	}
}

func TestReceiptMarkSent(t *testing.T) {
	r := newDraftReceipt()
	_ = r.AddLine("tx-1")
	_ = r.Finalize(1, time.Now())

	now := time.Now()
	if err := r.MarkSent(now, "customer@example.com"); err != nil {
		t.Fatalf("MarkSent returned error: %v", err)
	}
	if r.Status != ReceiptStatusSent {
		t.Fatalf("Status = %q, want %q", r.Status, ReceiptStatusSent)
	}
	if r.SentAt == nil || !r.SentAt.Equal(now) {
		t.Fatalf("SentAt = %v, want %v", r.SentAt, now)
	}
	if r.SentTo != "customer@example.com" {
		t.Fatalf("SentTo = %q, want %q", r.SentTo, "customer@example.com")
	}
}

func TestSentReceiptRejectsReopenAndMutation(t *testing.T) {
	r := newDraftReceipt()
	_ = r.AddLine("tx-1")
	_ = r.Finalize(1, time.Now())
	_ = r.MarkSent(time.Now(), "customer@example.com")

	if err := r.Reopen(); !errors.Is(err, ErrReceiptAlreadySent) {
		t.Fatalf("Reopen on sent receipt returned %v, want %v", err, ErrReceiptAlreadySent)
	}
	if err := r.MarkSent(time.Now(), "again@example.com"); !errors.Is(err, ErrReceiptAlreadySent) {
		t.Fatalf("double MarkSent returned %v, want %v", err, ErrReceiptAlreadySent)
	}
	if err := r.AddLine("tx-2"); !errors.Is(err, ErrReceiptFinalized) {
		t.Fatalf("AddLine on sent receipt returned %v, want %v", err, ErrReceiptFinalized)
	}
}

func TestMarkSentRejectsDraft(t *testing.T) {
	r := newDraftReceipt()
	if err := r.MarkSent(time.Now(), "customer@example.com"); !errors.Is(err, ErrReceiptNotFinalized) {
		t.Fatalf("MarkSent on draft returned %v, want %v", err, ErrReceiptNotFinalized)
	}
}
