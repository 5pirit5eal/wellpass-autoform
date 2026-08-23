package processor

import (
	"context"
	"errors"
	"testing"

	"github.com/wellpass-autoform/receipts-function/internal/bigquery"
	"github.com/wellpass-autoform/receipts-function/internal/config"
	"github.com/wellpass-autoform/receipts-function/internal/extractor"
)

type mockExtractor struct {
	meta *extractor.ReceiptMetadata
	err  error
}

func (m *mockExtractor) ExtractReceipt(ctx context.Context, fileBytes []byte, mimeType string) (*extractor.ReceiptMetadata, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.meta, nil
}

type mockStorage struct {
	existingObjects map[string]bool // key: "bucket/object"
	uploadedObjects map[string]mockUpload
	deletedObjects  map[string]bool
	readData        []byte
	readContentType string
	readErr         error
}

type mockUpload struct {
	Bucket      string
	ObjectName  string
	Data        []byte
	ContentType string
	Metadata    map[string]string
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		existingObjects: make(map[string]bool),
		uploadedObjects: make(map[string]mockUpload),
		deletedObjects:  make(map[string]bool),
	}
}

func (s *mockStorage) ObjectExists(ctx context.Context, bucket, objectName string) (bool, error) {
	key := bucket + "/" + objectName
	return s.existingObjects[key], nil
}

func (s *mockStorage) UploadReceipt(ctx context.Context, bucket, objectName string, data []byte, contentType string, metadata map[string]string) error {
	key := bucket + "/" + objectName
	s.uploadedObjects[key] = mockUpload{
		Bucket:      bucket,
		ObjectName:  objectName,
		Data:        data,
		ContentType: contentType,
		Metadata:    metadata,
	}
	return nil
}

func (s *mockStorage) ReadObject(ctx context.Context, bucket, objectName string) ([]byte, string, error) {
	if s.readErr != nil {
		return nil, "", s.readErr
	}
	return s.readData, s.readContentType, nil
}

func (s *mockStorage) DeleteObject(ctx context.Context, bucket, objectName string) error {
	key := bucket + "/" + objectName
	s.deletedObjects[key] = true
	return nil
}

func TestProcessSuccess(t *testing.T) {
	cfg := &config.Config{
		SourceBucket: "unprocessed-bucket",
		TargetBucket: "target-bucket",
		FailedBucket: "failed-bucket",
	}

	mockExt := &mockExtractor{
		meta: &extractor.ReceiptMetadata{
			Date:          "2026-08-07",
			TicketPrice:   5.44,
			Currency:      "EUR",
			Location:      "Schwimm in Bilk",
			ReceiptNumber: "104307524/1",
		},
	}

	mockStore := newMockStorage()
	mockBQ := &bigquery.NoopRecorder{}
	proc := NewReceiptProcessor(cfg, mockExt, mockStore, mockBQ)

	req := ProcessRequest{
		Data:             []byte("%PDF-1.4 mock content"),
		Filename:         "receipt.pdf",
		ContentType:      "application/pdf",
		SourceBucket:     "unprocessed-bucket",
		SourceObjectName: "receipt.pdf",
	}

	res, err := proc.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected process error: %v", err)
	}

	if res.Status != "processed" {
		t.Errorf("expected status 'processed', got '%s'", res.Status)
	}
	if res.Bucket != "target-bucket" {
		t.Errorf("expected bucket 'target-bucket', got '%s'", res.Bucket)
	}
	if res.ObjectName != "receipt.pdf" {
		t.Errorf("expected object 'receipt.pdf', got '%s'", res.ObjectName)
	}

	// Verify target bucket upload
	uploadKey := "target-bucket/receipt.pdf"
	upload, ok := mockStore.uploadedObjects[uploadKey]
	if !ok {
		t.Fatalf("object was not uploaded to target bucket")
	}
	if upload.Metadata["location"] != "Schwimm in Bilk" {
		t.Errorf("expected location 'Schwimm in Bilk', got '%s'", upload.Metadata["location"])
	}
	if upload.Metadata["date"] != "2026-08-07" {
		t.Errorf("expected date '2026-08-07', got '%s'", upload.Metadata["date"])
	}
	if upload.Metadata["ticket_price"] != "5.44" {
		t.Errorf("expected ticket_price '5.44', got '%s'", upload.Metadata["ticket_price"])
	}

	// Verify deletion from unprocessed source bucket
	if !mockStore.deletedObjects["unprocessed-bucket/receipt.pdf"] {
		t.Errorf("expected unprocessed source object to be deleted after successful processing")
	}

	// Verify BigQuery recording
	if len(mockBQ.Records) != 1 {
		t.Fatalf("expected 1 BigQuery record, got %d", len(mockBQ.Records))
	}
	if mockBQ.Records[0].Status != "processed" {
		t.Errorf("expected BQ status 'processed', got %s", mockBQ.Records[0].Status)
	}
	if !mockBQ.Records[0].SubmissionStatus.Valid || mockBQ.Records[0].SubmissionStatus.StringVal != "pending" {
		t.Errorf("expected BQ submission status 'pending', got %v", mockBQ.Records[0].SubmissionStatus)
	}
}

func TestProcessConflict(t *testing.T) {
	cfg := &config.Config{
		SourceBucket: "unprocessed-bucket",
		TargetBucket: "target-bucket",
		FailedBucket: "failed-bucket",
	}

	mockExt := &mockExtractor{
		meta: &extractor.ReceiptMetadata{
			Date:          "2026-08-07",
			TicketPrice:   5.44,
			Currency:      "EUR",
			Location:      "Schwimm in Bilk",
			ReceiptNumber: "104307524/1",
		},
	}

	mockStore := newMockStorage()
	mockBQ := &bigquery.NoopRecorder{}
	// Mark object as existing in target bucket to trigger conflict
	mockStore.existingObjects["target-bucket/receipt.pdf"] = true

	proc := NewReceiptProcessor(cfg, mockExt, mockStore, mockBQ)

	req := ProcessRequest{
		Data:             []byte("%PDF-1.4 mock content"),
		Filename:         "receipt.pdf",
		ContentType:      "application/pdf",
		SourceBucket:     "unprocessed-bucket",
		SourceObjectName: "receipt.pdf",
	}

	res, err := proc.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected process error: %v", err)
	}

	if res.Status != "conflict" {
		t.Errorf("expected status 'conflict', got '%s'", res.Status)
	}
	if res.Bucket != "failed-bucket" {
		t.Errorf("expected bucket 'failed-bucket', got '%s'", res.Bucket)
	}

	// Verify failed bucket upload
	uploadKey := "failed-bucket/receipt.pdf"
	upload, ok := mockStore.uploadedObjects[uploadKey]
	if !ok {
		t.Fatalf("conflicting object was not uploaded to failed bucket")
	}
	if upload.Metadata["status"] != "conflict" {
		t.Errorf("expected metadata status 'conflict', got '%s'", upload.Metadata["status"])
	}
	if upload.Metadata["conflict_reason"] == "" {
		t.Errorf("expected metadata conflict_reason to be non-empty")
	}

	// Verify deletion from unprocessed source bucket on conflict
	if !mockStore.deletedObjects["unprocessed-bucket/receipt.pdf"] {
		t.Errorf("expected unprocessed source object to be deleted after conflict upload")
	}

	// Verify BigQuery recording
	if len(mockBQ.Records) != 1 {
		t.Fatalf("expected 1 BigQuery record, got %d", len(mockBQ.Records))
	}
	if mockBQ.Records[0].Status != "conflict" {
		t.Errorf("expected BQ status 'conflict', got %s", mockBQ.Records[0].Status)
	}
}

func TestProcessExtractionFailure(t *testing.T) {
	cfg := &config.Config{
		SourceBucket: "unprocessed-bucket",
		TargetBucket: "target-bucket",
		FailedBucket: "failed-bucket",
	}

	mockExt := &mockExtractor{
		err: errors.New("gemini failed to parse document"),
	}

	mockStore := newMockStorage()
	mockBQ := &bigquery.NoopRecorder{}
	proc := NewReceiptProcessor(cfg, mockExt, mockStore, mockBQ)

	req := ProcessRequest{
		Data:             []byte("corrupted data"),
		Filename:         "bad.pdf",
		ContentType:      "application/pdf",
		SourceBucket:     "unprocessed-bucket",
		SourceObjectName: "bad.pdf",
	}

	res, err := proc.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Status != "failed" {
		t.Errorf("expected status 'failed', got '%s'", res.Status)
	}
	if res.Bucket != "failed-bucket" {
		t.Errorf("expected bucket 'failed-bucket', got '%s'", res.Bucket)
	}

	uploadKey := "failed-bucket/bad.pdf"
	upload, ok := mockStore.uploadedObjects[uploadKey]
	if !ok {
		t.Fatalf("failed object was not uploaded to failed bucket")
	}
	if upload.Metadata["status"] != "failed" {
		t.Errorf("expected status 'failed', got '%s'", upload.Metadata["status"])
	}
	if upload.Metadata["error"] == "" {
		t.Errorf("expected error metadata to be set")
	}

	// Verify deletion from unprocessed source bucket on failure
	if !mockStore.deletedObjects["unprocessed-bucket/bad.pdf"] {
		t.Errorf("expected unprocessed source object to be deleted after failure upload")
	}

	// Verify BigQuery recording
	if len(mockBQ.Records) != 1 {
		t.Fatalf("expected 1 BigQuery record, got %d", len(mockBQ.Records))
	}
	if mockBQ.Records[0].Status != "failed" {
		t.Errorf("expected BQ status 'failed', got %s", mockBQ.Records[0].Status)
	}
}
