package domain

import (
	"fmt"
	"time"
)

// ReceiptStatus tracks whether a receipt can still be edited.
type ReceiptStatus string

const (
	// ReceiptStatusDraft receipts are mutable: line items (transactions) can
	// be added or removed at any point, e.g. to correct a mistake.
	ReceiptStatusDraft ReceiptStatus = "draft"
	// ReceiptStatusFinalized receipts are locked and immutable, ready to be
	// emailed or printed.
	ReceiptStatusFinalized ReceiptStatus = "finalized"
)

// Receipt accumulates a customer's transactions during one visit. While in
// draft status its TransactionIDs can be freely added to or removed from; a
// vendor mistake (wrong product, bad weigh) is corrected by removing a line,
// not by editing the underlying Transaction. Once finalized it is immutable
// and gets a sequential, tenant-scoped Number for display on the receipt.
type Receipt struct {
	ID       string
	TenantID string
	UserID   string
	Status   ReceiptStatus

	// Number is a sequential, tenant-scoped receipt number assigned at
	// finalization. Zero until finalized.
	Number int

	TransactionIDs []string

	CreatedAt   time.Time
	FinalizedAt *time.Time
}

// ErrReceiptFinalized is returned when trying to mutate a finalized receipt.
var ErrReceiptFinalized = fmt.Errorf("receipt is finalized and can no longer be modified")

// AddLine appends a transaction to a draft receipt's line items.
func (r *Receipt) AddLine(transactionID string) error {
	if r.Status != ReceiptStatusDraft {
		return ErrReceiptFinalized
	}
	r.TransactionIDs = append(r.TransactionIDs, transactionID)
	return nil
}

// RemoveLine removes one occurrence of transactionID from a draft receipt's
// line items. It reports whether a matching line was found and removed.
func (r *Receipt) RemoveLine(transactionID string) (bool, error) {
	if r.Status != ReceiptStatusDraft {
		return false, ErrReceiptFinalized
	}
	for i, id := range r.TransactionIDs {
		if id == transactionID {
			r.TransactionIDs = append(r.TransactionIDs[:i], r.TransactionIDs[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}

// Finalize locks the receipt and assigns it the given sequential number. The
// caller is responsible for allocating a correct, tenant-scoped, gap-free
// number (see the storage layer).
func (r *Receipt) Finalize(number int, at time.Time) error {
	if r.Status != ReceiptStatusDraft {
		return ErrReceiptFinalized
	}
	r.Status = ReceiptStatusFinalized
	r.Number = number
	r.FinalizedAt = &at
	return nil
}
