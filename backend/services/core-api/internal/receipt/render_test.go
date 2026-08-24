package receipt

import (
	"strings"
	"testing"
	"time"
)

func sampleData() Data {
	return Data{
		TenantName:  "Farmers Market Stand",
		Number:      42,
		FinalizedAt: time.Date(2026, 8, 23, 14, 30, 0, 0, time.UTC),
		Lines: []LineData{
			{ProductName: "Tomatoes", WeightGrams: 1250, UnitPriceCents: 499, TotalPriceCents: 624},
			{ProductName: "Potatoes", WeightGrams: 2000, UnitPriceCents: 199, TotalPriceCents: 398},
		},
	}
}

func TestDataTotalCents(t *testing.T) {
	if got := sampleData().TotalCents(); got != 1022 {
		t.Fatalf("TotalCents() = %d, want 1022", got)
	}
}

func TestRenderTextContainsAllLines(t *testing.T) {
	text := RenderText(sampleData())
	for _, want := range []string{"Farmers Market Stand", "Receipt #42", "Tomatoes", "1.250 kg", "4.99/kg", "6.24", "Potatoes", "Total: 10.22"} {
		if !strings.Contains(text, want) {
			t.Fatalf("RenderText output missing %q; got:\n%s", want, text)
		}
	}
}

func TestRenderHTMLEscapesUntrustedNames(t *testing.T) {
	d := sampleData()
	d.TenantName = `<script>alert(1)</script>`
	d.Lines[0].ProductName = `Tom & Jerry's "Tomatoes"`

	htmlOut := RenderHTML(d)
	if strings.Contains(htmlOut, "<script>alert(1)</script>") {
		t.Fatal("RenderHTML did not escape a tenant name containing markup")
	}
	if strings.Contains(htmlOut, `Tom & Jerry's "Tomatoes"`) {
		t.Fatal("RenderHTML did not escape a product name containing special characters")
	}
	if !strings.Contains(htmlOut, "&lt;script&gt;") {
		t.Fatal("expected the tenant name to appear HTML-escaped")
	}
}

func TestRenderHTMLContainsTotal(t *testing.T) {
	htmlOut := RenderHTML(sampleData())
	if !strings.Contains(htmlOut, "10.22") {
		t.Fatalf("RenderHTML output missing total; got:\n%s", htmlOut)
	}
}
