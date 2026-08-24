package bigquery

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	"github.com/wellpass-autoform/receipts-function/internal/config"
	"github.com/wellpass-autoform/receipts-function/internal/extractor"
)

// ProcessingRecord represents a row in the unified BigQuery processing_results table.
type ProcessingRecord struct {
	ReceiptID         string               `bigquery:"receipt_id"`
	SourceFilename    string               `bigquery:"source_filename"`
	Date              bigquery.NullDate    `bigquery:"date"`
	TicketPrice       bigquery.NullFloat64 `bigquery:"ticket_price"`
	Currency          bigquery.NullString  `bigquery:"currency"`
	Location          bigquery.NullString  `bigquery:"location"`
	ReceiptNumber     bigquery.NullString  `bigquery:"receipt_number"`
	CustomerName      bigquery.NullString  `bigquery:"customer_name"`
	TicketType        bigquery.NullString  `bigquery:"ticket_type"`
	Status            string               `bigquery:"status"` // "processed", "conflict", "failed"
	DestinationBucket string               `bigquery:"destination_bucket"`
	ConflictReason    bigquery.NullString  `bigquery:"conflict_reason"`
	ErrorMessage      bigquery.NullString  `bigquery:"error_message"`
	RawMetadata       bigquery.NullString  `bigquery:"raw_metadata"` // JSON string
	ProcessedAt       time.Time            `bigquery:"processed_at"`
	SubmissionStatus  bigquery.NullString  `bigquery:"submission_status"`
	LastUpdatedAt     time.Time            `bigquery:"last_updated_at"`
}

// BuildRecord creates a ProcessingRecord from extraction metadata and processing outcome.
func BuildRecord(
	filename, status, bucket, conflictReason, errMsg string,
	meta *extractor.ReceiptMetadata,
	processedAt time.Time,
) *ProcessingRecord {
	if processedAt.IsZero() {
		processedAt = time.Now().UTC()
	}

	receiptID := fmt.Sprintf("rec_%s_%s", filename, processedAt.Format("20060102150405"))

	rec := &ProcessingRecord{
		ReceiptID:         receiptID,
		SourceFilename:    filename,
		Status:            status,
		DestinationBucket: bucket,
		ProcessedAt:       processedAt,
		LastUpdatedAt:     processedAt,
	}

	if conflictReason != "" {
		rec.ConflictReason = bigquery.NullString{StringVal: conflictReason, Valid: true}
	}
	if errMsg != "" {
		rec.ErrorMessage = bigquery.NullString{StringVal: errMsg, Valid: true}
	}

	if status == "processed" {
		rec.SubmissionStatus = bigquery.NullString{StringVal: "pending", Valid: true}
	}

	if meta != nil {
		if meta.Date != "" {
			if parsedDate, err := civil.ParseDate(meta.Date); err == nil {
				rec.Date = bigquery.NullDate{Date: parsedDate, Valid: true}
			}
		}
		if meta.TicketPrice > 0 {
			rec.TicketPrice = bigquery.NullFloat64{Float64: meta.TicketPrice, Valid: true}
		}
		if meta.Currency != "" {
			rec.Currency = bigquery.NullString{StringVal: meta.Currency, Valid: true}
		}
		if meta.Location != "" {
			rec.Location = bigquery.NullString{StringVal: meta.Location, Valid: true}
		}
		if meta.ReceiptNumber != "" {
			rec.ReceiptNumber = bigquery.NullString{StringVal: meta.ReceiptNumber, Valid: true}
		}
		if meta.TicketType != "" {
			rec.TicketType = bigquery.NullString{StringVal: meta.TicketType, Valid: true}
		}
		if rawJSON, err := json.Marshal(meta); err == nil {
			rec.RawMetadata = bigquery.NullString{StringVal: string(rawJSON), Valid: true}
		}
	}

	return rec
}

// Recorder defines the interface for recording processing metrics to BigQuery.
type Recorder interface {
	Record(ctx context.Context, rec *ProcessingRecord) error
	Close() error
}

// BQRecorder implements Recorder using the Google Cloud BigQuery Go SDK.
type BQRecorder struct {
	client   *bigquery.Client
	dataset  string
	table    string
	inserter *bigquery.Inserter
}

// NewBQRecorder creates a new BQRecorder for the configured project, dataset, and table.
func NewBQRecorder(ctx context.Context, cfg *config.Config) (*BQRecorder, error) {
	if cfg.ProjectID == "" || cfg.BigQueryDataset == "" || cfg.BigQueryTable == "" {
		return nil, fmt.Errorf("project ID, BigQuery dataset, and BigQuery table must be configured")
	}

	client, err := bigquery.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create BigQuery client: %w", err)
	}

	table := client.Dataset(cfg.BigQueryDataset).Table(cfg.BigQueryTable)
	inserter := table.Inserter()

	return &BQRecorder{
		client:   client,
		dataset:  cfg.BigQueryDataset,
		table:    cfg.BigQueryTable,
		inserter: inserter,
	}, nil
}

// Record inserts a processing record into BigQuery.
func (r *BQRecorder) Record(ctx context.Context, rec *ProcessingRecord) error {
	if rec == nil {
		return nil
	}
	if err := r.inserter.Put(ctx, []*ProcessingRecord{rec}); err != nil {
		return fmt.Errorf("failed to stream insert record into BigQuery %s.%s: %w", r.dataset, r.table, err)
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

// NoopRecorder is a mock recorder that logs or ignores writes (useful for testing or fallback).
type NoopRecorder struct {
	Records []*ProcessingRecord
}

// Record appends the record in memory.
func (n *NoopRecorder) Record(ctx context.Context, rec *ProcessingRecord) error {
	if rec != nil {
		n.Records = append(n.Records, rec)
	}
	return nil
}

// Close is a no-op.
func (n *NoopRecorder) Close() error {
	return nil
}
