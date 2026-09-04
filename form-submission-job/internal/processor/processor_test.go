package processor

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/wellpass-autoform/form-submission-job/internal/bigquery"
	"github.com/wellpass-autoform/form-submission-job/internal/config"
	"github.com/wellpass-autoform/form-submission-job/internal/matcher"
	"github.com/wellpass-autoform/form-submission-job/internal/storage"
	"github.com/wellpass-autoform/form-submission-job/internal/submitter"
)

func TestJobProcessorWorkflow(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "job-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	mockItems := []*storage.ReceiptItem{
		{
			Bucket:      "test-bucket",
			ObjectName:  "receipt1.pdf",
			Date:        "2026-06-15", // 2 months ago (allowed)
			TicketPrice: 5.44,
			Location:    "Schwimm in Bilk",
			Status:      "processed",
			CreatedAt:   time.Now(),
		},
		{
			Bucket:      "test-bucket",
			ObjectName:  "receipt2.pdf",
			Date:        "2026-07-20", // 1 month ago (allowed)
			TicketPrice: 4.50,
			Location:    "Münster Therme",
			Status:      "processed",
			CreatedAt:   time.Now(),
		},
		{
			Bucket:      "test-bucket",
			ObjectName:  "receipt3.pdf",
			Date:        "2026-08-01", // Current submission month (allowed)
			TicketPrice: 6.00,
			Location:    "Freizeitbad Düsselstrand",
			Status:      "processed",
			CreatedAt:   time.Now(),
		},
		{
			Bucket:      "test-bucket",
			ObjectName:  "old_receipt.pdf",
			Date:        "2026-05-01", // 3+ months ago (out of allowed window)
			TicketPrice: 5.00,
			Location:    "Rheinbad Düsseldorf",
			Status:      "processed",
			CreatedAt:   time.Now(),
		},
	}

	cfg := &config.Config{
		SourceBucket:    "test-processed-bucket",
		SubmittedBucket: "test-submitted-bucket",
		FailedBucket:    "test-failed-bucket",
		Email:           "max.mustermann@example.com",
		FirstName:       "Max",
		LastName:        "Mustermann",
		IBAN:            "DE1234567890",
		BIC:             "GENODEM1GLS",
		DryRun:          false,
		TargetMonth:     "2026-08",
	}

	mockStore := storage.NewMockStorageService(mockItems)
	mockMatch := matcher.NewPoolMatcher(nil)
	mockSub := &submitter.MockSubmitter{}
	mockBQ := &bigquery.MockRecorder{}

	mockShot := "/tmp/mock_screenshot.png"
	_ = os.WriteFile(mockShot, []byte("fake png content"), 0644)
	defer func() {
		_ = os.Remove(mockShot)
	}()

	proc := NewJobProcessor(cfg, mockStore, mockMatch, mockSub, tempDir, mockBQ)

	report, err := proc.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}

	if report.TotalDiscovered != 3 {
		t.Errorf("got %d discovered, want 3", report.TotalDiscovered)
	}
	if report.TotalSubmitted != 3 {
		t.Errorf("got %d submitted, want 3", report.TotalSubmitted)
	}
	if report.BatchesCount != 1 {
		t.Errorf("got %d batches, want 1", report.BatchesCount)
	}
	if len(mockStore.SubmittedList) != 3 {
		t.Errorf("expected 3 items moved to submitted bucket, got %d", len(mockStore.SubmittedList))
	} else {
		if mockStore.SubmittedList[0] != "2026-08/receipt1.pdf" {
			t.Errorf("expected destination object 2026-08/receipt1.pdf, got %s", mockStore.SubmittedList[0])
		}
		if mockStore.SubmittedList[1] != "2026-08/receipt2.pdf" {
			t.Errorf("expected destination object 2026-08/receipt2.pdf, got %s", mockStore.SubmittedList[1])
		}
		if mockStore.SubmittedList[2] != "2026-08/receipt3.pdf" {
			t.Errorf("expected destination object 2026-08/receipt3.pdf, got %s", mockStore.SubmittedList[2])
		}
	}
	if len(mockStore.DeletedList) != 3 {
		t.Errorf("expected 3 items deleted from processed bucket, got %d", len(mockStore.DeletedList))
	}
	if len(mockStore.UploadedFiles) != 1 {
		t.Errorf("expected 1 screenshot uploaded to failed bucket, got %d", len(mockStore.UploadedFiles))
	}

	// Verify BigQuery updates
	if len(mockBQ.Updates) != 3 {
		t.Fatalf("expected 3 BigQuery submission updates, got %d", len(mockBQ.Updates))
	}
	for _, u := range mockBQ.Updates {
		if u.SubmissionStatus != "submitted" {
			t.Errorf("expected submission status 'submitted', got %s", u.SubmissionStatus)
		}
		if u.SubmissionMonth != "2026-08" {
			t.Errorf("expected submission month '2026-08', got %s", u.SubmissionMonth)
		}
		if u.ArchiveGCSURI == "" {
			t.Errorf("expected non-empty archive GCS URI for %s", u.SourceFilename)
		}
	}
}

func TestChunkTickets(t *testing.T) {
	var tickets []submitter.SubmissionTicket
	for i := 0; i < 25; i++ {
		tickets = append(tickets, submitter.SubmissionTicket{ObjectName: "r.pdf"})
	}

	chunks := chunkTickets(tickets, 10)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks for 25 items with chunk size 10, got %d", len(chunks))
	}
	if len(chunks[0]) != 10 || len(chunks[1]) != 10 || len(chunks[2]) != 5 {
		t.Errorf("unexpected chunk sizes: %d, %d, %d", len(chunks[0]), len(chunks[1]), len(chunks[2]))
	}
}

func TestJobProcessorFailureScreenshotMetadata(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "job-fail-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	mockShot := "/tmp/mock_screenshot.png"
	_ = os.WriteFile(mockShot, []byte("fake screenshot data"), 0644)
	defer func() {
		_ = os.Remove(mockShot)
	}()

	mockItems := []*storage.ReceiptItem{
		{
			Bucket:      "test-bucket",
			ObjectName:  "receipt1.pdf",
			Date:        "2026-08-10",
			TicketPrice: 5.00,
			Location:    "Schwimm in Bilk",
			Status:      "processed",
			CreatedAt:   time.Now(),
		},
	}

	mockStore := storage.NewMockStorageService(mockItems)
	mockPoolMatcher := matcher.NewPoolMatcher(nil)
	failingSubmitter := &submitter.MockSubmitter{ShouldFail: true}
	mockRec := &bigquery.MockRecorder{}

	cfg := &config.Config{
		ProjectID:       "test-project",
		SourceBucket:    "test-processed-bucket",
		SubmittedBucket: "test-submitted-bucket",
		FailedBucket:    "test-failed-bucket",
		TargetMonth:     "2026-08",
		DryRun:          false,
	}

	proc := NewJobProcessor(cfg, mockStore, mockPoolMatcher, failingSubmitter, tempDir, mockRec)
	report, err := proc.Run(context.Background())
	if err == nil {
		t.Fatalf("expected error from failing submitter, got nil")
	}
	if report == nil {
		t.Fatalf("expected report to be returned even on failure")
	}

	// Verify screenshots have failure metadata attached
	foundScreenshot := false
	for k, meta := range mockStore.UploadedMeta {
		if strings.Contains(k, "screenshots/2026-08/") {
			foundScreenshot = true
			if meta["status"] != "failed" {
				t.Errorf("expected metadata status 'failed', got %q", meta["status"])
			}
			if meta["error_message"] != "mock submit error" {
				t.Errorf("expected error_message 'mock submit error', got %q", meta["error_message"])
			}
			if meta["receipt_files"] != "receipt1.pdf" {
				t.Errorf("expected receipt_files 'receipt1.pdf', got %q", meta["receipt_files"])
			}
		}
	}
	if !foundScreenshot {
		t.Errorf("expected screenshot upload with metadata to be recorded in mockStore.UploadedMeta")
	}
}
