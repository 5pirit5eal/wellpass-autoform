package extractor

import (
	"testing"
)

func TestReceiptMetadataValidate(t *testing.T) {
	tests := []struct {
		name    string
		meta    ReceiptMetadata
		wantErr bool
	}{
		{
			name: "Valid metadata with ISO date",
			meta: ReceiptMetadata{
				Date:          "2026-08-07",
				TicketPrice:   5.44,
				Currency:      "EUR",
				Location:      "Schwimm in Bilk",
				ReceiptNumber: "104307524/1",
			},
			wantErr: false,
		},
		{
			name: "Valid metadata with German date",
			meta: ReceiptMetadata{
				Date:        "07.08.2026",
				TicketPrice: 5.44,
				Currency:    "EUR",
				Location:    "Schwimm in Bilk",
			},
			wantErr: false,
		},
		{
			name: "Missing date",
			meta: ReceiptMetadata{
				Date:        "",
				TicketPrice: 5.44,
				Location:    "Schwimm in Bilk",
			},
			wantErr: true,
		},
		{
			name: "Zero ticket price",
			meta: ReceiptMetadata{
				Date:        "2026-08-07",
				TicketPrice: 0.0,
				Location:    "Schwimm in Bilk",
			},
			wantErr: true,
		},
		{
			name: "Negative ticket price",
			meta: ReceiptMetadata{
				Date:        "2026-08-07",
				TicketPrice: -5.44,
				Location:    "Schwimm in Bilk",
			},
			wantErr: true,
		},
		{
			name: "Missing location",
			meta: ReceiptMetadata{
				Date:        "2026-08-07",
				TicketPrice: 5.44,
				Location:    "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.meta.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestReceiptMetadataToMetadataMap(t *testing.T) {
	meta := ReceiptMetadata{
		Date:          "07.08.2026",
		TicketPrice:   5.44,
		Currency:      "EUR",
		Location:      "Schwimm in Bilk",
		ReceiptNumber: "104307524/1",
		CustomerName:  "Ruben Schulze",
		TicketType:    "Erwachsene 1",
	}

	mapMeta := meta.ToMetadataMap()

	if mapMeta["date"] != "2026-08-07" {
		t.Errorf("expected date 2026-08-07, got %s", mapMeta["date"])
	}
	if mapMeta["ticket_price"] != "5.44" {
		t.Errorf("expected ticket_price 5.44, got %s", mapMeta["ticket_price"])
	}
	if mapMeta["currency"] != "EUR" {
		t.Errorf("expected currency EUR, got %s", mapMeta["currency"])
	}
	if mapMeta["location"] != "Schwimm in Bilk" {
		t.Errorf("expected location 'Schwimm in Bilk', got %s", mapMeta["location"])
	}
	if mapMeta["receipt_number"] != "104307524/1" {
		t.Errorf("expected receipt_number '104307524/1', got %s", mapMeta["receipt_number"])
	}
	if mapMeta["customer_name"] != "Ruben Schulze" {
		t.Errorf("expected customer_name 'Ruben Schulze', got %s", mapMeta["customer_name"])
	}
	if mapMeta["ticket_type"] != "Erwachsene 1" {
		t.Errorf("expected ticket_type 'Erwachsene 1', got %s", mapMeta["ticket_type"])
	}
	if mapMeta["structured_data"] == "" {
		t.Errorf("expected structured_data to be populated")
	}
}
