package processor

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/wellpass-autoform/form-submission-job/internal/bigquery"
	"github.com/wellpass-autoform/form-submission-job/internal/config"
	"github.com/wellpass-autoform/form-submission-job/internal/matcher"
	"github.com/wellpass-autoform/form-submission-job/internal/storage"
	"github.com/wellpass-autoform/form-submission-job/internal/submitter"
)

// RunReport summarizes the overall job run execution.
type RunReport struct {
	TargetMonth       string                        `json:"target_month"`
	TotalDiscovered   int                           `json:"total_discovered"`
	TotalSubmitted    int                           `json:"total_submitted"`
	BatchesCount      int                           `json:"batches_count"`
	DryRun            bool                          `json:"dry_run"`
	BatchResults      []*submitter.SubmissionResult `json:"batch_results"`
	UnmatchedReceipts []string                      `json:"unmatched_receipts,omitempty"`
}

// JobProcessor coordinates retrieval, matching, chunking, and submission.
type JobProcessor struct {
	cfg       *config.Config
	storage   storage.StorageService
	matcher   *matcher.PoolMatcher
	submitter submitter.FormSubmitter
	tempDir   string
	recorder  bigquery.Recorder
}

// NewJobProcessor creates a new JobProcessor.
func NewJobProcessor(
	cfg *config.Config,
	store storage.StorageService,
	match *matcher.PoolMatcher,
	sub submitter.FormSubmitter,
	tempDir string,
	rec ...bigquery.Recorder,
) *JobProcessor {
	if tempDir == "" {
		tempDir = filepath.Join(os.TempDir(), "wellpass-submissions")
	}
	var recorder bigquery.Recorder
	if len(rec) > 0 && rec[0] != nil {
		recorder = rec[0]
	}
	return &JobProcessor{
		cfg:       cfg,
		storage:   store,
		matcher:   match,
		submitter: sub,
		tempDir:   tempDir,
		recorder:  recorder,
	}
}

// Run executes the full monthly submission workflow.
func (p *JobProcessor) Run(ctx context.Context) (*RunReport, error) {
	submissionMonth := p.cfg.TargetSubmissionMonth()
	allowedMonths := p.cfg.AllowedReceiptMonths()
	log.Printf("Starting form submission job for submission month: %s (eligible receipt window: %v, Bucket: %s, DryRun: %v)",
		submissionMonth, allowedMonths, p.cfg.SourceBucket, p.cfg.DryRun)

	// Step 1: List processed receipts from GCS matching the 3-month eligibility window
	receipts, err := p.storage.ListProcessedReceipts(ctx, p.cfg.SourceBucket, allowedMonths)
	if err != nil {
		return nil, fmt.Errorf("failed to list processed receipts: %w", err)
	}

	report := &RunReport{
		TargetMonth:     submissionMonth,
		TotalDiscovered: len(receipts),
		DryRun:          p.cfg.DryRun,
		BatchResults:    []*submitter.SubmissionResult{},
	}

	if len(receipts) == 0 {
		log.Printf("No un-submitted receipts found for submission month %s (window: %v).", submissionMonth, allowedMonths)
		return report, nil
	}

	log.Printf("Found %d eligible receipt(s) to process for submission month %s.", len(receipts), submissionMonth)

	// Ensure temporary directory exists
	if err := os.MkdirAll(p.tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create working temp dir %s: %w", p.tempDir, err)
	}
	defer func() {
		_ = os.RemoveAll(p.tempDir)
	}()

	// Step 2: Match pools and download files
	var preparedTickets []submitter.SubmissionTicket

	for _, r := range receipts {
		day, mon, yr, dateErr := r.SplitDate()
		if dateErr != nil {
			log.Printf("Skipping receipt %s: invalid date %q (%v)", r.ObjectName, r.Date, dateErr)
			continue
		}

		matchRes, matchErr := p.matcher.Match(r.Location)
		if matchErr != nil {
			log.Printf("Warning: Failed to match pool location %q for receipt %s: %v", r.Location, r.ObjectName, matchErr)
			report.UnmatchedReceipts = append(report.UnmatchedReceipts, fmt.Sprintf("%s: %s", r.ObjectName, r.Location))
			if !p.cfg.DryRun {
				if moveErr := p.storage.MoveToFailed(ctx, p.cfg.SourceBucket, p.cfg.FailedBucket, r.ObjectName, submissionMonth, matchErr.Error()); moveErr != nil {
					log.Printf("Warning: failed to move unmatched receipt %s to failed bucket: %v", r.ObjectName, moveErr)
				}
			}
			p.recordUnmatched(ctx, r.ObjectName, submissionMonth, matchErr.Error())
			continue
		}

		// Download receipt file to local disk for upload
		localPath := filepath.Join(p.tempDir, r.ObjectName)
		if err := p.storage.DownloadReceipt(ctx, r.Bucket, r.ObjectName, localPath); err != nil {
			return nil, fmt.Errorf("failed to download receipt %s: %w", r.ObjectName, err)
		}

		preparedTickets = append(preparedTickets, submitter.SubmissionTicket{
			ObjectName:    r.ObjectName,
			PoolLabel:     matchRes.MatchedLabel,
			Day:           day,
			Month:         mon,
			Year:          yr,
			Price:         r.FormatPriceGerman(),
			FilePath:      localPath,
			ReceiptNumber: r.ReceiptNumber,
		})
	}

	if len(preparedTickets) == 0 {
		return report, fmt.Errorf("no valid tickets could be prepared for submission")
	}

	// Step 3: Chunk into batches of up to 10 tickets
	batches := chunkTickets(preparedTickets, 10)
	report.BatchesCount = len(batches)
	log.Printf("Chunked %d valid ticket(s) into %d batch(es) for submission.", len(preparedTickets), len(batches))

	// Step 4: Submit each batch
	for idx, batchTickets := range batches {
		batchID := fmt.Sprintf("sub_%s_batch%d_%s", submissionMonth, idx+1, time.Now().Format("150405"))
		subBatch := submitter.SubmissionBatch{
			BatchID:        batchID,
			TypeformURL:    p.cfg.TypeformURL,
			Email:          p.cfg.Email,
			FullName:       p.cfg.FullName(),
			IBAN:           p.cfg.IBAN,
			BIC:            p.cfg.BIC,
			Tickets:        batchTickets,
			DryRun:         p.cfg.DryRun,
			Headless:       p.cfg.Headless,
			ScreenshotsDir: p.cfg.ScreenshotsDir,
		}

		log.Printf("Submitting batch %d/%d (Batch ID: %s, %d tickets)...", idx+1, len(batches), batchID, len(batchTickets))
		subResult, err := p.submitter.Submit(ctx, subBatch)
		var screenshotURIs []string
		if subResult != nil {
			report.BatchResults = append(report.BatchResults, subResult)
			screenshotURIs = p.uploadBatchScreenshots(ctx, submissionMonth, batchID, subResult.Screenshots)
		}
		if err != nil {
			p.recordBatchFailure(ctx, batchTickets, submissionMonth, batchID, err.Error(), screenshotURIs)
			return report, fmt.Errorf("submission failed for batch %s: %w", batchID, err)
		}

		if subResult.Success {
			report.TotalSubmitted += len(batchTickets)
			// Move receipts to submitted archive bucket inside configured submission month folder
			if !p.cfg.DryRun {
				for _, t := range batchTickets {
					if moveErr := p.storage.MoveToSubmitted(ctx, p.cfg.SourceBucket, p.cfg.SubmittedBucket, t.ObjectName, submissionMonth, batchID); moveErr != nil {
						log.Printf("Warning: failed to move %s to submitted archive bucket %s (%s): %v", t.ObjectName, p.cfg.SubmittedBucket, submissionMonth, moveErr)
					}
				}
			}
			p.recordBatchSuccess(ctx, batchTickets, submissionMonth, batchID, screenshotURIs)
		}
	}

	log.Printf("Submission job complete. Total submitted: %d/%d tickets in %d batch(es).",
		report.TotalSubmitted, report.TotalDiscovered, report.BatchesCount)

	return report, nil
}

func (p *JobProcessor) uploadBatchScreenshots(ctx context.Context, month, batchID string, screenshots []string) []string {
	if len(screenshots) == 0 || p.cfg.FailedBucket == "" {
		return nil
	}
	log.Printf("Uploading %d screenshot(s) to inspection bucket gs://%s/screenshots/%s/%s/...", len(screenshots), p.cfg.FailedBucket, month, batchID)
	var uris []string
	for _, sPath := range screenshots {
		if sPath == "" {
			continue
		}
		base := filepath.Base(sPath)
		targetObj := fmt.Sprintf("screenshots/%s/%s/%s", month, batchID, base)
		meta := map[string]string{
			"batch_id":    batchID,
			"month":       month,
			"uploaded_at": time.Now().UTC().Format(time.RFC3339),
		}
		if err := p.storage.UploadFile(ctx, p.cfg.FailedBucket, targetObj, sPath, "image/png", meta); err != nil {
			log.Printf("Warning: failed to upload screenshot %s to gs://%s/%s: %v", sPath, p.cfg.FailedBucket, targetObj, err)
		} else {
			uris = append(uris, fmt.Sprintf("gs://%s/%s", p.cfg.FailedBucket, targetObj))
		}
	}
	return uris
}

func (p *JobProcessor) recordUnmatched(ctx context.Context, filename, month, errMsg string) {
	if p.recorder == nil {
		return
	}
	update := &bigquery.SubmissionUpdate{
		SourceFilename:   filename,
		SubmissionStatus: "unmatched_pool",
		SubmissionMonth:  month,
		IsDryRun:         p.cfg.DryRun,
		SubmissionError:  errMsg,
		LastUpdatedAt:    time.Now().UTC(),
	}
	if err := p.recorder.RecordSubmission(ctx, update); err != nil {
		log.Printf("Warning: failed to record unmatched receipt %s to BigQuery: %v", filename, err)
	}
}

func (p *JobProcessor) recordBatchSuccess(ctx context.Context, tickets []submitter.SubmissionTicket, month, batchID string, screenshotURIs []string) {
	if p.recorder == nil {
		return
	}
	status := "submitted"
	if p.cfg.DryRun {
		status = "dry_run_success"
	}
	now := time.Now().UTC()

	var updates []*bigquery.SubmissionUpdate
	for _, t := range tickets {
		archiveURI := fmt.Sprintf("gs://%s/%s/%s", p.cfg.SubmittedBucket, month, t.ObjectName)
		updates = append(updates, &bigquery.SubmissionUpdate{
			SourceFilename:   t.ObjectName,
			SubmissionStatus: status,
			SubmissionMonth:  month,
			BatchID:          batchID,
			MatchedPoolLabel: t.PoolLabel,
			MatcherScore:     1.0,
			IsDryRun:         p.cfg.DryRun,
			SubmittedAt:      now,
			ArchiveGCSURI:    archiveURI,
			ScreenshotURIs:   screenshotURIs,
			LastUpdatedAt:    now,
		})
	}

	if err := p.recorder.RecordBatchSubmissions(ctx, updates); err != nil {
		log.Printf("Warning: failed to record batch %s submissions to BigQuery: %v", batchID, err)
	}
}

func (p *JobProcessor) recordBatchFailure(ctx context.Context, tickets []submitter.SubmissionTicket, month, batchID, errMsg string, screenshotURIs []string) {
	if p.recorder == nil {
		return
	}
	now := time.Now().UTC()
	var updates []*bigquery.SubmissionUpdate
	for _, t := range tickets {
		updates = append(updates, &bigquery.SubmissionUpdate{
			SourceFilename:   t.ObjectName,
			SubmissionStatus: "submission_failed",
			SubmissionMonth:  month,
			BatchID:          batchID,
			MatchedPoolLabel: t.PoolLabel,
			IsDryRun:         p.cfg.DryRun,
			SubmissionError:  errMsg,
			ScreenshotURIs:   screenshotURIs,
			LastUpdatedAt:    now,
		})
	}
	if err := p.recorder.RecordBatchSubmissions(ctx, updates); err != nil {
		log.Printf("Warning: failed to record batch %s failure to BigQuery: %v", batchID, err)
	}
}

func chunkTickets(tickets []submitter.SubmissionTicket, chunkSize int) [][]submitter.SubmissionTicket {
	var chunks [][]submitter.SubmissionTicket
	for i := 0; i < len(tickets); i += chunkSize {
		end := i + chunkSize
		if end > len(tickets) {
			end = len(tickets)
		}
		chunks = append(chunks, tickets[i:end])
	}
	return chunks
}
