package handler

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/wellpass-autoform/receipts-function/internal/config"
	"github.com/wellpass-autoform/receipts-function/internal/processor"
	"github.com/wellpass-autoform/receipts-function/internal/storage"
)

// StorageObjectData represents the payload of a Cloud Storage CloudEvent.
type StorageObjectData struct {
	Bucket         string `json:"bucket"`
	Name           string `json:"name"`
	Metageneration string `json:"metageneration,omitempty"`
	TimeCreated    string `json:"timeCreated,omitempty"`
	Updated        string `json:"updated,omitempty"`
	ContentType    string `json:"contentType,omitempty"`
	Size           string `json:"size,omitempty"`
}

// CloudEventHandler handles CloudEvent triggers from Google Cloud Storage.
type CloudEventHandler struct {
	cfg       *config.Config
	processor *processor.ReceiptProcessor
	storage   storage.StorageService
}

// NewCloudEventHandler creates a new CloudEventHandler.
func NewCloudEventHandler(cfg *config.Config, proc *processor.ReceiptProcessor, store storage.StorageService) *CloudEventHandler {
	return &CloudEventHandler{
		cfg:       cfg,
		processor: proc,
		storage:   store,
	}
}

// HandleCloudEvent processes incoming GCS finalized events.
func (h *CloudEventHandler) HandleCloudEvent(ctx context.Context, e cloudevents.Event) error {
	var data StorageObjectData
	if err := e.DataAs(&data); err != nil {
		return fmt.Errorf("failed to decode cloudevent storage data: %w", err)
	}

	if data.Bucket == "" || data.Name == "" {
		log.Printf("Ignoring event with missing bucket or object name: bucket=%s name=%s", data.Bucket, data.Name)
		return nil
	}

	// Prevent event loops: ignore objects in target or failed buckets
	if data.Bucket == h.cfg.TargetBucket || data.Bucket == h.cfg.FailedBucket {
		log.Printf("Ignoring event from processed/failed bucket gs://%s/%s", data.Bucket, data.Name)
		return nil
	}

	log.Printf("Processing CloudEvent for gs://%s/%s (type: %s)", data.Bucket, data.Name, e.Type())

	fileBytes, contentType, err := h.storage.ReadObject(ctx, data.Bucket, data.Name)
	if err != nil {
		log.Printf("Error processing receipt gs://%s/%s: %v", data.Bucket, data.Name, err)
		return fmt.Errorf("failed to read source object gs://%s/%s: %w", data.Bucket, data.Name, err)
	}

	if contentType == "" {
		contentType = data.ContentType
	}

	filename := filepath.Base(data.Name)

	req := processor.ProcessRequest{
		Data:             fileBytes,
		Filename:         filename,
		ContentType:      contentType,
		SourceBucket:     data.Bucket,
		SourceObjectName: data.Name,
	}

	result, err := h.processor.Process(ctx, req)
	if err != nil {
		log.Printf("Error processing receipt gs://%s/%s: %v", data.Bucket, data.Name, err)
		return fmt.Errorf("failed to process receipt gs://%s/%s: %w", data.Bucket, data.Name, err)
	}

	log.Printf("Completed receipt processing for %s: status=%s, destination=gs://%s/%s", filename, result.Status, result.Bucket, result.ObjectName)
	return nil
}
