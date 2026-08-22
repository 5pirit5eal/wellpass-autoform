package handler

import (
	"context"
	"net/http"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/wellpass-autoform/receipts-function/internal/config"
	"github.com/wellpass-autoform/receipts-function/internal/extractor"
	"github.com/wellpass-autoform/receipts-function/internal/processor"
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
	existingObjects map[string]bool
	uploadedObjects map[string]mockUpload
	deletedObjects  map[string]bool
	objects         map[string][]byte
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
		objects:         make(map[string][]byte),
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
	key := bucket + "/" + objectName
	data, ok := s.objects[key]
	if !ok {
		return nil, "", http.ErrMissingFile
	}
	return data, "application/pdf", nil
}

func (s *mockStorage) DeleteObject(ctx context.Context, bucket, objectName string) error {
	key := bucket + "/" + objectName
	s.deletedObjects[key] = true
	return nil
}

func TestCloudEventHandlerSuccess(t *testing.T) {
	cfg := &config.Config{
		SourceBucket: "incoming-bucket",
		TargetBucket: "target-bkt",
		FailedBucket: "failed-bkt",
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
	store := newMockStorage()
	store.objects["incoming-bucket/receipt-1.pdf"] = []byte("%PDF-1.4 incoming data")

	proc := processor.NewReceiptProcessor(cfg, mockExt, store)
	ceHandler := NewCloudEventHandler(cfg, proc, store)

	event := cloudevents.NewEvent()
	event.SetID("12345")
	event.SetType("google.cloud.storage.object.v1.finalized")
	event.SetSource("//storage.googleapis.com/projects/_/buckets/incoming-bucket")
	_ = event.SetData(cloudevents.ApplicationJSON, StorageObjectData{
		Bucket:      "incoming-bucket",
		Name:        "receipt-1.pdf",
		ContentType: "application/pdf",
	})

	err := ceHandler.HandleCloudEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("HandleCloudEvent failed: %v", err)
	}

	upload, ok := store.uploadedObjects["target-bkt/receipt-1.pdf"]
	if !ok {
		t.Fatalf("expected receipt-1.pdf to be uploaded to target-bkt")
	}
	if upload.Metadata["location"] != "Schwimm in Bilk" {
		t.Errorf("expected location 'Schwimm in Bilk', got %s", upload.Metadata["location"])
	}

	if !store.deletedObjects["incoming-bucket/receipt-1.pdf"] {
		t.Errorf("expected incoming-bucket/receipt-1.pdf to be deleted after processing")
	}
}

func TestCloudEventHandlerIgnoreLoops(t *testing.T) {
	cfg := &config.Config{
		SourceBucket: "incoming-bucket",
		TargetBucket: "target-bkt",
		FailedBucket: "failed-bkt",
	}

	store := newMockStorage()
	proc := processor.NewReceiptProcessor(cfg, &mockExtractor{}, store)
	ceHandler := NewCloudEventHandler(cfg, proc, store)

	// Event for target bucket (should be ignored to prevent loops)
	event := cloudevents.NewEvent()
	event.SetID("12345")
	event.SetType("google.cloud.storage.object.v1.finalized")
	_ = event.SetData(cloudevents.ApplicationJSON, StorageObjectData{
		Bucket: "target-bkt",
		Name:   "processed.pdf",
	})

	err := ceHandler.HandleCloudEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error for ignored bucket: %v", err)
	}

	if len(store.uploadedObjects) != 0 {
		t.Errorf("expected no upload actions for ignored event")
	}
}
