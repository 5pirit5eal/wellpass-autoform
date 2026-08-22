package storage

import (
	"context"
	"errors"
	"fmt"
	"io"

	"cloud.google.com/go/storage"
)

// GCSStorage implements StorageService using the Google Cloud Storage Go SDK.
type GCSStorage struct {
	client *storage.Client
}

// NewGCSStorage initializes a new GCS storage service.
func NewGCSStorage(ctx context.Context) (*GCSStorage, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}
	return &GCSStorage{client: client}, nil
}

// NewGCSStorageWithClient initializes a GCS storage service with an existing client.
func NewGCSStorageWithClient(client *storage.Client) *GCSStorage {
	return &GCSStorage{client: client}
}

// Close closes the underlying GCS client.
func (s *GCSStorage) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

// ObjectExists checks whether an object exists in a given bucket.
func (s *GCSStorage) ObjectExists(ctx context.Context, bucket, objectName string) (bool, error) {
	if bucket == "" || objectName == "" {
		return false, fmt.Errorf("bucket name and object name must not be empty")
	}

	_, err := s.client.Bucket(bucket).Object(objectName).Attrs(ctx)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, storage.ErrObjectNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("failed to check existence of gs://%s/%s: %w", bucket, objectName, err)
}

// UploadReceipt writes receipt data and attaches structured metadata.
func (s *GCSStorage) UploadReceipt(ctx context.Context, bucket, objectName string, data []byte, contentType string, metadata map[string]string) error {
	if bucket == "" || objectName == "" {
		return fmt.Errorf("bucket name and object name must not be empty")
	}

	obj := s.client.Bucket(bucket).Object(objectName)
	writer := obj.NewWriter(ctx)
	if contentType != "" {
		writer.ContentType = contentType
	}
	if metadata != nil {
		writer.Metadata = metadata
	}

	if _, err := writer.Write(data); err != nil {
		_ = writer.Close()
		return fmt.Errorf("failed to write data to gs://%s/%s: %w", bucket, objectName, err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to finalize upload to gs://%s/%s: %w", bucket, objectName, err)
	}

	return nil
}

// ReadObject reads object content from GCS.
func (s *GCSStorage) ReadObject(ctx context.Context, bucket, objectName string) ([]byte, string, error) {
	if bucket == "" || objectName == "" {
		return nil, "", fmt.Errorf("bucket name and object name must not be empty")
	}

	reader, err := s.client.Bucket(bucket).Object(objectName).NewReader(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create reader for gs://%s/%s: %w", bucket, objectName, err)
	}
	defer func() { _ = reader.Close() }()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read data from gs://%s/%s: %w", bucket, objectName, err)
	}

	contentType := reader.Attrs.ContentType
	return data, contentType, nil
}

// DeleteObject deletes an object from a GCS bucket.
func (s *GCSStorage) DeleteObject(ctx context.Context, bucket, objectName string) error {
	if bucket == "" || objectName == "" {
		return fmt.Errorf("bucket name and object name must not be empty")
	}

	err := s.client.Bucket(bucket).Object(objectName).Delete(ctx)
	if err != nil && !errors.Is(err, storage.ErrObjectNotExist) {
		return fmt.Errorf("failed to delete gs://%s/%s: %w", bucket, objectName, err)
	}
	return nil
}
