package bigquery

import (
	"context"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/wellpass-autoform/form-submission-job/internal/config"
)

// SubmissionUpdate contains the submission status and metadata for a single receipt.
type SubmissionUpdate struct {
	SourceFilename   string
	CustomerName     string
	SubmissionStatus string // "submitted", "dry_run_success", "unmatched_pool", "submission_failed"
	SubmissionMonth  string
	BatchID          string
	MatchedPoolLabel string
	MatcherScore     float64
	IsDryRun         bool
	SubmittedAt      time.Time
	ArchiveGCSURI    string
	ScreenshotURIs   []string
	SubmissionError  string
	LastUpdatedAt    time.Time
}

// Recorder defines the interface for updating submission metrics in BigQuery.
type Recorder interface {
	RecordSubmission(ctx context.Context, update *SubmissionUpdate) error
	RecordBatchSubmissions(ctx context.Context, updates []*SubmissionUpdate) error
	Close() error
}

// BQRecorder implements Recorder using the Google Cloud BigQuery Go SDK.
type BQRecorder struct {
	client  *bigquery.Client
	dataset string
	table   string
}

// NewBQRecorder initializes a new BigQuery submission recorder.
func NewBQRecorder(ctx context.Context, cfg *config.Config) (*BQRecorder, error) {
	if cfg.ProjectID == "" || cfg.BigQueryDataset == "" || cfg.BigQueryTable == "" {
		return nil, fmt.Errorf("project ID, BigQuery dataset, and BigQuery table must be configured")
	}

	client, err := bigquery.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create BigQuery client: %w", err)
	}

	return &BQRecorder{
		client:  client,
		dataset: cfg.BigQueryDataset,
		table:   cfg.BigQueryTable,
	}, nil
}

// RecordSubmission updates a single receipt's submission status in BigQuery.
func (r *BQRecorder) RecordSubmission(ctx context.Context, u *SubmissionUpdate) error {
	if u == nil {
		return nil
	}
	return r.RecordBatchSubmissions(ctx, []*SubmissionUpdate{u})
}

// RecordBatchSubmissions updates multiple receipts' submission status in BigQuery using MERGE queries.
func (r *BQRecorder) RecordBatchSubmissions(ctx context.Context, updates []*SubmissionUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	for _, u := range updates {
		if u == nil || u.SourceFilename == "" {
			continue
		}
		if u.LastUpdatedAt.IsZero() {
			u.LastUpdatedAt = time.Now().UTC()
		}

		mergeSQL := fmt.Sprintf(`
MERGE %s.%s T
USING (
  SELECT
    @source_filename AS source_filename,
    @customer_name AS customer_name,
    @submission_status AS submission_status,
    @submission_month AS submission_month,
    @batch_id AS batch_id,
    @matched_pool_label AS matched_pool_label,
    @matcher_score AS matcher_score,
    @is_dry_run AS is_dry_run,
    @submitted_at AS submitted_at,
    @archive_gcs_uri AS archive_gcs_uri,
    @screenshot_uris AS screenshot_uris,
    @submission_error AS submission_error,
    @last_updated_at AS last_updated_at
) S
ON T.source_filename = S.source_filename
WHEN MATCHED THEN
  UPDATE SET
    customer_name = COALESCE(NULLIF(S.customer_name, ''), T.customer_name),
    submission_status = S.submission_status,
    submission_month = S.submission_month,
    batch_id = S.batch_id,
    matched_pool_label = S.matched_pool_label,
    matcher_score = S.matcher_score,
    is_dry_run = S.is_dry_run,
    submitted_at = S.submitted_at,
    archive_gcs_uri = S.archive_gcs_uri,
    screenshot_uris = S.screenshot_uris,
    submission_error = S.submission_error,
    last_updated_at = S.last_updated_at
WHEN NOT MATCHED THEN
  INSERT (
    receipt_id,
    source_filename,
    customer_name,
    status,
    destination_bucket,
    submission_status,
    submission_month,
    batch_id,
    matched_pool_label,
    matcher_score,
    is_dry_run,
    submitted_at,
    archive_gcs_uri,
    screenshot_uris,
    submission_error,
    processed_at,
    last_updated_at
  )
  VALUES (
    GENERATE_UUID(),
    S.source_filename,
    S.customer_name,
    'processed',
    'unknown',
    S.submission_status,
    S.submission_month,
    S.batch_id,
    S.matched_pool_label,
    S.matcher_score,
    S.is_dry_run,
    S.submitted_at,
    S.archive_gcs_uri,
    S.screenshot_uris,
    S.submission_error,
    S.last_updated_at,
    S.last_updated_at
  )
`, r.dataset, r.table)

		q := r.client.Query(mergeSQL)
		q.Parameters = []bigquery.QueryParameter{
			{Name: "source_filename", Value: u.SourceFilename},
			{Name: "customer_name", Value: u.CustomerName},
			{Name: "submission_status", Value: u.SubmissionStatus},
			{Name: "submission_month", Value: u.SubmissionMonth},
			{Name: "batch_id", Value: u.BatchID},
			{Name: "matched_pool_label", Value: u.MatchedPoolLabel},
			{Name: "matcher_score", Value: u.MatcherScore},
			{Name: "is_dry_run", Value: u.IsDryRun},
			{Name: "submitted_at", Value: u.SubmittedAt},
			{Name: "archive_gcs_uri", Value: u.ArchiveGCSURI},
			{Name: "screenshot_uris", Value: u.ScreenshotURIs},
			{Name: "submission_error", Value: u.SubmissionError},
			{Name: "last_updated_at", Value: u.LastUpdatedAt},
		}

		job, err := q.Run(ctx)
		if err != nil {
			log.Printf("Warning: failed to execute BigQuery MERGE for receipt %s: %v", u.SourceFilename, err)
			continue
		}

		status, err := job.Wait(ctx)
		if err != nil || (status != nil && status.Err() != nil) {
			log.Printf("Warning: BigQuery MERGE job failed for receipt %s: %v", u.SourceFilename, err)
		}
	}

	return nil
}

// Close closes the BigQuery client.
func (r *BQRecorder) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// MockRecorder is an in-memory recorder for tests.
type MockRecorder struct {
	Updates []*SubmissionUpdate
}

// RecordSubmission saves the update in memory.
func (m *MockRecorder) RecordSubmission(ctx context.Context, update *SubmissionUpdate) error {
	if update != nil {
		m.Updates = append(m.Updates, update)
	}
	return nil
}

// RecordBatchSubmissions saves multiple updates in memory.
func (m *MockRecorder) RecordBatchSubmissions(ctx context.Context, updates []*SubmissionUpdate) error {
	for _, u := range updates {
		if u != nil {
			m.Updates = append(m.Updates, u)
		}
	}
	return nil
}

// Close is a no-op.
func (m *MockRecorder) Close() error {
	return nil
}
