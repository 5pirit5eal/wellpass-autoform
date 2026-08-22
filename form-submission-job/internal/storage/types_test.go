package storage

import (
	"testing"
)

func TestReceiptItemFormatting(t *testing.T) {
	item := &ReceiptItem{
		Date:        "2026-08-15",
		TicketPrice: 5.44,
		Location:    "Schwimm in Bilk",
	}

	if germanPrice := item.FormatPriceGerman(); germanPrice != "5,44" {
		t.Errorf("got %q, want %q", germanPrice, "5,44")
	}

	day, month, year, err := item.SplitDate()
	if err != nil {
		t.Fatalf("unexpected error splitting date: %v", err)
	}
	if day != "15" || month != "08" || year != "2026" {
		t.Errorf("got %s/%s/%s, want 15/08/2026", day, month, year)
	}
}
