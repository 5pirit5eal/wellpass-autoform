package processor

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/wellpass-autoform/receipts-function/internal/bigquery"
	"github.com/wellpass-autoform/receipts-function/internal/config"
	"github.com/wellpass-autoform/receipts-function/internal/extractor"
	"github.com/wellpass-autoform/receipts-function/internal/storage"
)

// ProcessRequest contains the receipt data and attributes to process.
type ProcessRequest struct {
	Data             []byte `json:"-"`
	Filename         string `json:"filename"`
	ContentType      string `json:"content_type"`
	SourceBucket     string `json:"source_bucket,omitempty"`
	SourceObjectName string `json:"source_object_name,omitempty"`
}

// ProcessResult describes the outcome of receipt processing and storage.
type ProcessResult struct {
	Status         string                     `json:"status"` // "processed", "conflict", or "failed"
	Bucket         string                     `json:"bucket"`
	ObjectName     string                     `json:"object_name"`
	Metadata       *extractor.ReceiptMetadata `json:"metadata,omitempty"`
	ConflictReason string                     `json:"conflict_reason,omitempty"`
	Error          string                     `json:"error,omitempty"`
}

// ReceiptProcessor orchestrates extraction, conflict checking, and bucket uploads.
type ReceiptProcessor struct {
	cfg       *config.Config
	extractor extractor.ReceiptExtractor
	storage   storage.StorageService
	recorder  bigquery.Recorder
}

// NewReceiptProcessor creates a new ReceiptProcessor instance.
func NewReceiptProcessor(cfg *config.Config, ext extractor.ReceiptExtractor, store storage.StorageService, rec ...bigquery.Recorder) *ReceiptProcessor {
	var recorder bigquery.Recorder
	if len(rec) > 0 && rec[0] != nil {
		recorder = rec[0]
	}
	return &ReceiptProcessor{
		cfg:       cfg,
		extractor: ext,
		storage:   store,
		recorder:  recorder,
	}
}

// Process handles receipt processing workflow:
// 1. Validates input
// 2. Extracts structured output via Gemini
// 3. Checks for conflicts in target bucket
// 4. If conflict or failure -> uploads to FailedBucket
// 5. If clean -> uploads to TargetBucket with metadata attached
// 6. Records analytics to BigQuery
func (p *ReceiptProcessor) Process(ctx context.Context, req ProcessRequest) (*ProcessResult, error) {
	if len(req.Data) == 0 {
		return nil, fmt.Errorf("receipt file data is empty")
	}

	// Sanitize and ensure filename
	req.Filename = sanitizeFilename(req.Filename)
	if req.Filename == "" {
		ext := ".pdf"
		if strings.Contains(req.ContentType, "text") {
			ext = ".txt"
		}
		req.Filename = fmt.Sprintf("receipt_%s%s", time.Now().UTC().Format("20060102_150405"), ext)
	}

	// Normalize content type
	if req.ContentType == "" || req.ContentType == "application/octet-stream" {
		if len(req.Data) >= 4 && string(req.Data[:4]) == "%PDF" {
			req.ContentType = "application/pdf"
		} else {
			req.ContentType = "text/plain"
		}
	}

	now := time.Now().UTC()

	// Step 1: Extract structured data with Gemini
	meta, err := p.extractor.ExtractReceipt(ctx, req.Data, req.ContentType)
	if err != nil {
		// Extraction failed -> upload to FailedBucket
		failedMeta := map[string]string{
			"status":            "failed",
			"error":             err.Error(),
			"processed_at":      now.Format(time.RFC3339),
			"original_filename": req.Filename,
		}

		uploadErr := p.storage.UploadReceipt(ctx, p.cfg.FailedBucket, req.Filename, req.Data, req.ContentType, failedMeta)
		if uploadErr != nil {
			return nil, fmt.Errorf("failed extraction (%w) and failed to upload to failed bucket: %v", err, uploadErr)
		}

		p.deleteSourceIfPresent(ctx, req)
		p.recordAnalytics(ctx, req.Filename, "failed", p.cfg.FailedBucket, "", err.Error(), nil, now)

		return &ProcessResult{
			Status:     "failed",
			Bucket:     p.cfg.FailedBucket,
			ObjectName: req.Filename,
			Error:      err.Error(),
		}, nil
	}

	// Step 2: Conflict Check - does the object already exist in TargetBucket?
	exists, checkErr := p.storage.ObjectExists(ctx, p.cfg.TargetBucket, req.Filename)
	if checkErr != nil {
		return nil, fmt.Errorf("failed to check conflict in target bucket %s: %w", p.cfg.TargetBucket, checkErr)
	}

	if exists {
		// Conflict detected -> upload to FailedBucket
		conflictReason := fmt.Sprintf("file %q already exists in target bucket %s", req.Filename, p.cfg.TargetBucket)
		conflictMeta := meta.ToMetadataMap()
		conflictMeta["status"] = "conflict"
		conflictMeta["conflict_reason"] = conflictReason
		conflictMeta["processed_at"] = now.Format(time.RFC3339)
		conflictMeta["original_filename"] = req.Filename

		uploadErr := p.storage.UploadReceipt(ctx, p.cfg.FailedBucket, req.Filename, req.Data, req.ContentType, conflictMeta)
		if uploadErr != nil {
			return nil, fmt.Errorf("conflict detected (%s) but failed to upload to failed bucket: %w", conflictReason, uploadErr)
		}

		p.deleteSourceIfPresent(ctx, req)
		p.recordAnalytics(ctx, req.Filename, "conflict", p.cfg.FailedBucket, conflictReason, "", meta, now)

		return &ProcessResult{
			Status:         "conflict",
			Bucket:         p.cfg.FailedBucket,
			ObjectName:     req.Filename,
			Metadata:       meta,
			ConflictReason: conflictReason,
		}, nil
	}

	// Step 3: No conflict -> Upload to TargetBucket with structured metadata
	targetMeta := meta.ToMetadataMap()
	targetMeta["status"] = "processed"
	targetMeta["processed_at"] = now.Format(time.RFC3339)
	targetMeta["original_filename"] = req.Filename

	uploadErr := p.storage.UploadReceipt(ctx, p.cfg.TargetBucket, req.Filename, req.Data, req.ContentType, targetMeta)
	if uploadErr != nil {
		return nil, fmt.Errorf("failed to upload receipt to target bucket %s: %w", p.cfg.TargetBucket, uploadErr)
	}

	p.deleteSourceIfPresent(ctx, req)
	p.recordAnalytics(ctx, req.Filename, "processed", p.cfg.TargetBucket, "", "", meta, now)

	return &ProcessResult{
		Status:     "processed",
		Bucket:     p.cfg.TargetBucket,
		ObjectName: req.Filename,
		Metadata:   meta,
	}, nil
}

func (p *ReceiptProcessor) recordAnalytics(
	ctx context.Context,
	filename, status, bucket, conflictReason, errMsg string,
	meta *extractor.ReceiptMetadata,
	processedAt time.Time,
) {
	if p.recorder == nil {
		return
	}
	rec := bigquery.BuildRecord(filename, status, bucket, conflictReason, errMsg, meta, processedAt)
	if err := p.recorder.Record(ctx, rec); err != nil {
		log.Printf("Warning: failed to record processing analytics to BigQuery for %s: %v", filename, err)
	}
}

func (p *ReceiptProcessor) deleteSourceIfPresent(ctx context.Context, req ProcessRequest) {
	if req.SourceBucket == "" || req.SourceObjectName == "" {
		return
	}
	// Do not delete if source bucket is target or failed bucket
	if req.SourceBucket == p.cfg.TargetBucket || req.SourceBucket == p.cfg.FailedBucket {
		return
	}
	if err := p.storage.DeleteObject(ctx, req.SourceBucket, req.SourceObjectName); err != nil {
		log.Printf("Warning: failed to delete unprocessed source object gs://%s/%s: %v", req.SourceBucket, req.SourceObjectName, err)
	}
}

func sanitizeFilename(name string) string {
	clean := filepath.Base(strings.TrimSpace(name))
	if clean == "." || clean == "/" || clean == "\\" {
		return ""
	}
	return clean
}
