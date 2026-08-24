package bigquery

import (
	"context"
	"testing"
	"time"

	"github.com/wellpass-autoform/receipts-function/internal/extractor"
)

func TestBuildRecord(t *testing.T) {
	meta := &extractor.ReceiptMetadata{
		Date:          "2026-08-15",
		TicketPrice:   5.44,
		Currency:      "EUR",
		Location:      "Schwimm in Bilk",
		ReceiptNumber: "REC-12345",
		TicketType:    "Einzelkarte",
	}

	now := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)
	rec := BuildRecord("sample.pdf", "processed", "my-target-bucket", "", "", meta, now)

	if rec.SourceFilename != "sample.pdf" {
		t.Errorf("expected source filename 'sample.pdf', got %q", rec.SourceFilename)
	}
	if rec.Status != "processed" {
		t.Errorf("expected status 'processed', got %q", rec.Status)
	}
	if rec.DestinationBucket != "my-target-bucket" {
		t.Errorf("expected destination bucket 'my-target-bucket', got %q", rec.DestinationBucket)
	}
	if !rec.Date.Valid || rec.Date.Date.String() != "2026-08-15" {
		t.Errorf("expected valid date 2026-08-15, got %v", rec.Date)
	}
	if !rec.TicketPrice.Valid || rec.TicketPrice.Float64 != 5.44 {
		t.Errorf("expected ticket price 5.44, got %v", rec.TicketPrice)
	}
	if !rec.SubmissionStatus.Valid || rec.SubmissionStatus.StringVal != "pending" {
		t.Errorf("expected submission_status 'pending', got %v", rec.SubmissionStatus)
	}
}

func TestNoopRecorder(t *testing.T) {
	ctx := context.Background()
	recorder := &NoopRecorder{}

	rec := BuildRecord("error.pdf", "failed", "my-failed-bucket", "", "Gemini timeout", nil, time.Now().UTC())
	if err := recorder.Record(ctx, rec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(recorder.Records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recorder.Records))
	}
	if recorder.Records[0].Status != "failed" {
		t.Errorf("expected status failed, got %s", recorder.Records[0].Status)
	}
}
