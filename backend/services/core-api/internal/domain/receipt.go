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
	// ReceiptStatusFinalized receipts are locked from editing and ready to
	// be sent (emailed or, later, printed) — but still reversible: Reopen
	// puts one back into draft, e.g. to correct a mis-scanned line.
	ReceiptStatusFinalized ReceiptStatus = "finalized"
	// ReceiptStatusSent receipts have actually been handed to the customer
	// (emailed or printed) and are locked for good — this, not
	// finalization, is the real point of no return.
	ReceiptStatusSent ReceiptStatus = "sent"
)

// Receipt accumulates a customer's transactions during one visit. While in
// draft status its TransactionIDs can be freely added to or removed from; a
// vendor mistake (wrong product, bad weigh) is corrected by removing a line,
// not by editing the underlying Transaction. Once finalized it is locked and
// gets a sequential, tenant-scoped Number for display on the receipt, but
// can still be reopened back into a draft. Once sent, it is permanently
// locked.
type Receipt struct {
	ID       string        `json:"id"`
	TenantID string        `json:"tenant_id"`
	UserID   string        `json:"user_id"`
	Status   ReceiptStatus `json:"status"`

	// Number is a sequential, tenant-scoped receipt number assigned at
	// finalization. Zero until finalized; cleared again if reopened, since
	// a draft shouldn't carry a stale number, and re-finalizing allocates a
	// fresh one.
	Number int `json:"number,omitempty"`

	TransactionIDs []string `json:"transaction_ids"`

	CreatedAt   time.Time  `json:"created_at"`
	FinalizedAt *time.Time `json:"finalized_at,omitempty"`

	// SentAt/SentTo are set once the receipt has actually been sent (see
	// MarkSent). SentTo is the destination (e.g. an email address).
	SentAt *time.Time `json:"sent_at,omitempty"`
	SentTo string     `json:"sent_to,omitempty"`
}

// ErrReceiptFinalized is returned when trying to mutate (add/remove a line,
// or finalize) a receipt that isn't a draft.
var ErrReceiptFinalized = fmt.Errorf("receipt is finalized and can no longer be modified")

// ErrReceiptNotFinalized is returned when trying to reopen or send a
// receipt that hasn't been finalized yet.
var ErrReceiptNotFinalized = fmt.Errorf("receipt is not finalized yet")

// ErrReceiptAlreadySent is returned when trying to reopen or re-send a
// receipt that has already been sent — the one state with no way back.
var ErrReceiptAlreadySent = fmt.Errorf("receipt has already been sent and can no longer be modified")

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

// Reopen puts a finalized receipt back into draft status, e.g. to correct a
// mis-scanned line. It clears Number and FinalizedAt: a draft shouldn't
// carry a stale receipt number, and re-finalizing allocates a fresh one. A
// receipt that has already been sent can no longer be reopened.
func (r *Receipt) Reopen() error {
	if r.Status == ReceiptStatusSent {
		return ErrReceiptAlreadySent
	}
	if r.Status != ReceiptStatusFinalized {
		return ErrReceiptNotFinalized
	}
	r.Status = ReceiptStatusDraft
	r.Number = 0
	r.FinalizedAt = nil
	return nil
}

// MarkSent locks a finalized receipt for good, recording when and where it
// was sent (e.g. emailed, or later printed). This, not Finalize, is the
// real point of no return.
func (r *Receipt) MarkSent(at time.Time, to string) error {
	if r.Status == ReceiptStatusSent {
		return ErrReceiptAlreadySent
	}
	if r.Status != ReceiptStatusFinalized {
		return ErrReceiptNotFinalized
	}
	r.Status = ReceiptStatusSent
	r.SentAt = &at
	r.SentTo = to
	return nil
}
