package processor

import (
	"context"
	"os"
	"testing"
	"time"

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
			Date:        "2026-08-05",
			TicketPrice: 5.44,
			Location:    "Schwimm in Bilk",
			Status:      "processed",
			CreatedAt:   time.Now(),
		},
		{
			Bucket:      "test-bucket",
			ObjectName:  "receipt2.pdf",
			Date:        "2026-08-12",
			TicketPrice: 4.50,
			Location:    "Münster Therme",
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

	proc := NewJobProcessor(cfg, mockStore, mockMatch, mockSub, tempDir)

	report, err := proc.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}

	if report.TotalDiscovered != 2 {
		t.Errorf("got %d discovered, want 2", report.TotalDiscovered)
	}
	if report.TotalSubmitted != 2 {
		t.Errorf("got %d submitted, want 2", report.TotalSubmitted)
	}
	if report.BatchesCount != 1 {
		t.Errorf("got %d batches, want 1", report.BatchesCount)
	}
	if len(mockStore.SubmittedList) != 2 {
		t.Errorf("expected 2 items moved to submitted bucket, got %d", len(mockStore.SubmittedList))
	} else if mockStore.SubmittedList[0] != "2026-08/receipt1.pdf" {
		t.Errorf("expected destination object 2026-08/receipt1.pdf, got %s", mockStore.SubmittedList[0])
	}
	if len(mockStore.DeletedList) != 2 {
		t.Errorf("expected 2 items deleted from processed bucket, got %d", len(mockStore.DeletedList))
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
