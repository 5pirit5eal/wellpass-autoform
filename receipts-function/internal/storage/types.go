package storage

import (
	"context"
)

// StorageService defines operations for interacting with Cloud Storage buckets.
type StorageService interface {
	// ObjectExists checks if an object with objectName already exists in the bucket.
	ObjectExists(ctx context.Context, bucket, objectName string) (bool, error)

	// UploadReceipt uploads the raw receipt bytes to the specified bucket with attached custom metadata.
	UploadReceipt(ctx context.Context, bucket, objectName string, data []byte, contentType string, metadata map[string]string) error

	// ReadObject reads the raw content and content-type of an object from a bucket.
	ReadObject(ctx context.Context, bucket, objectName string) ([]byte, string, error)

	// DeleteObject deletes an object from a bucket.
	DeleteObject(ctx context.Context, bucket, objectName string) error
}
