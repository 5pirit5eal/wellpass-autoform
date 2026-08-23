package bigquery

import (
	"context"
	"testing"
	"time"
)

func TestMockRecorder(t *testing.T) {
	ctx := context.Background()
	recorder := &MockRecorder{}

	update := &SubmissionUpdate{
		SourceFilename:   "receipt-1.pdf",
		SubmissionStatus: "submitted",
		SubmissionMonth:  "2026-08",
		BatchID:          "sub_2026-08_batch1_120000",
		MatchedPoolLabel: "Schwimm' in Bilk Düsseldorf",
		MatcherScore:     1.0,
		IsDryRun:         false,
		SubmittedAt:      time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC),
		ArchiveGCSURI:    "gs://my-submitted-bucket/2026-08/receipt-1.pdf",
		ScreenshotURIs:   []string{"gs://my-failed-bucket/screenshots/2026-08/sub_2026-08_batch1_120000/01_welcome.png"},
		LastUpdatedAt:    time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC),
	}

	if err := recorder.RecordSubmission(ctx, update); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(recorder.Updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(recorder.Updates))
	}

	if recorder.Updates[0].SourceFilename != "receipt-1.pdf" {
		t.Errorf("expected source filename 'receipt-1.pdf', got %q", recorder.Updates[0].SourceFilename)
	}
	if recorder.Updates[0].SubmissionStatus != "submitted" {
		t.Errorf("expected status 'submitted', got %q", recorder.Updates[0].SubmissionStatus)
	}
	if recorder.Updates[0].MatchedPoolLabel != "Schwimm' in Bilk Düsseldorf" {
		t.Errorf("expected pool 'Schwimm'' in Bilk Düsseldorf', got %q", recorder.Updates[0].MatchedPoolLabel)
	}

	// Test batch submissions
	batchUpdates := []*SubmissionUpdate{
		{
			SourceFilename:   "receipt-2.pdf",
			SubmissionStatus: "dry_run_success",
			SubmissionMonth:  "2026-08",
		},
	}
	if err := recorder.RecordBatchSubmissions(ctx, batchUpdates); err != nil {
		t.Fatalf("unexpected error on batch: %v", err)
	}
	if len(recorder.Updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(recorder.Updates))
	}
}
