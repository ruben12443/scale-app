// Package receipt renders a finalized receipt into printable/emailable
// output and sends it by email. It depends on no storage or domain types —
// callers build a Data value from whatever they have, keeping this package
// trivial to unit test and reusable if the caller-side data model changes.
package receipt

import (
	"fmt"
	"time"
)

// LineData is one line item on a rendered receipt.
type LineData struct {
	ProductName     string
	WeightGrams     int
	UnitPriceCents  int
	TotalPriceCents int
}

// Data is everything needed to render a finalized receipt.
type Data struct {
	TenantName  string
	Number      int
	FinalizedAt time.Time
	Lines       []LineData
}

// TotalCents sums every line's total.
func (d Data) TotalCents() int {
	total := 0
	for _, l := range d.Lines {
		total += l.TotalPriceCents
	}
	return total
}

// formatCents renders an integer cent amount as e.g. "4.99".
func formatCents(cents int) string {
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}

// formatWeight renders a gram amount as e.g. "1.250 kg".
func formatWeight(grams int) string {
	return fmt.Sprintf("%d.%03d kg", grams/1000, grams%1000)
}
