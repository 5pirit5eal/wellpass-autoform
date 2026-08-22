package storage

import (
	"context"
	"testing"
)

func TestStorageServiceValidation(t *testing.T) {
	var service StorageService = &GCSStorage{}

	ctx := context.Background()

	// Empty bucket or object validation
	_, err := service.ObjectExists(ctx, "", "test.pdf")
	if err == nil {
		t.Error("expected error when bucket is empty")
	}

	_, err = service.ObjectExists(ctx, "bucket", "")
	if err == nil {
		t.Error("expected error when objectName is empty")
	}

	err = service.UploadReceipt(ctx, "", "test.pdf", []byte("data"), "application/pdf", nil)
	if err == nil {
		t.Error("expected error when bucket is empty on upload")
	}

	_, _, err = service.ReadObject(ctx, "bucket", "")
	if err == nil {
		t.Error("expected error when objectName is empty on read")
	}

	err = service.DeleteObject(ctx, "", "test.pdf")
	if err == nil {
		t.Error("expected error when bucket is empty on delete")
	}

	err = service.DeleteObject(ctx, "bucket", "")
	if err == nil {
		t.Error("expected error when objectName is empty on delete")
	}
}
