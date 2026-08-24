package receipt

import (
	"fmt"
	"html"
	"strings"
)

// RenderText renders a plain-text receipt, suitable for a simple
// text-oriented thermal printer or a plain-text email fallback.
//
// Actual printer hardware integration (ESC/POS commands, CUPS, a network
// printer protocol, etc.) is out of scope here — this produces the text
// content a print pathway would send, once a target printer is chosen.
func RenderText(d Data) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", d.TenantName)
	fmt.Fprintf(&b, "Receipt #%d\n", d.Number)
	fmt.Fprintf(&b, "%s\n", d.FinalizedAt.Format("2006-01-02 15:04"))
	b.WriteString(strings.Repeat("-", 32) + "\n")
	for _, l := range d.Lines {
		fmt.Fprintf(&b, "%s\n", l.ProductName)
		fmt.Fprintf(&b, "  %s x %s/kg = %s\n", formatWeight(l.WeightGrams), formatCents(l.UnitPriceCents), formatCents(l.TotalPriceCents))
	}
	b.WriteString(strings.Repeat("-", 32) + "\n")
	fmt.Fprintf(&b, "Total: %s\n", formatCents(d.TotalCents()))
	return b.String()
}

// RenderHTML renders an HTML receipt, suitable for the body of an email.
func RenderHTML(d Data) string {
	var b strings.Builder
	b.WriteString("<html><body>\n")
	fmt.Fprintf(&b, "<h2>%s</h2>\n", html.EscapeString(d.TenantName))
	fmt.Fprintf(&b, "<p>Receipt #%d<br>%s</p>\n", d.Number, html.EscapeString(d.FinalizedAt.Format("2006-01-02 15:04")))
	b.WriteString("<table>\n<tbody>\n")
	for _, l := range d.Lines {
		fmt.Fprintf(&b, "<tr><td>%s</td><td>%s x %s/kg</td><td>%s</td></tr>\n",
			html.EscapeString(l.ProductName), formatWeight(l.WeightGrams), formatCents(l.UnitPriceCents), formatCents(l.TotalPriceCents))
	}
	b.WriteString("</tbody>\n</table>\n")
	fmt.Fprintf(&b, "<p><strong>Total: %s</strong></p>\n", formatCents(d.TotalCents()))
	b.WriteString("</body></html>\n")
	return b.String()
}
