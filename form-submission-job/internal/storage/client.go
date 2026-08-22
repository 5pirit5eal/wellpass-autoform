package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	gcs "cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

// GCSStorageService interacts with Google Cloud Storage.
type GCSStorageService struct {
	client *gcs.Client
}

// NewGCSStorageService creates a new GCSStorageService with a live GCS client.
func NewGCSStorageService(ctx context.Context) (*GCSStorageService, error) {
	client, err := gcs.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}
	return &GCSStorageService{client: client}, nil
}

// Close closes the underlying GCS client.
func (s *GCSStorageService) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

// ListProcessedReceipts returns all receipts in the given bucket matching monthFilter (e.g. "2026-08").
// If monthFilter is empty, all un-submitted receipts are returned.
func (s *GCSStorageService) ListProcessedReceipts(ctx context.Context, bucket string, monthFilter string) ([]*ReceiptItem, error) {
	var items []*ReceiptItem
	bkt := s.client.Bucket(bucket)
	it := bkt.Objects(ctx, nil)

	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate objects in bucket %s: %w", bucket, err)
		}

		// Only process valid receipt files
		if strings.HasSuffix(attrs.Name, "/") {
			continue
		}

		item := ParseMetadata(bucket, attrs.Name, attrs.Metadata, attrs.Created)

		// Filter already submitted receipts
		if item.Status == "submitted" || item.SubmittedAt != "" {
			continue
		}

		// Filter by month if provided (e.g. "2026-08" matches item.Date starting with "2026-08")
		if monthFilter != "" {
			if item.Date != "" && !strings.HasPrefix(item.Date, monthFilter) {
				continue
			}
		}

		items = append(items, item)
	}

	return items, nil
}

// DownloadReceipt downloads the object from GCS to a local file destination.
func (s *GCSStorageService) DownloadReceipt(ctx context.Context, bucket, objectName, destPath string) error {
	bkt := s.client.Bucket(bucket)
	obj := bkt.Object(objectName)

	rc, err := obj.NewReader(ctx)
	if err != nil {
		return fmt.Errorf("failed to open object gs://%s/%s for reading: %w", bucket, objectName, err)
	}
	defer func() {
		_ = rc.Close()
	}()

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", destPath, err)
	}
	defer func() {
		_ = out.Close()
	}()

	if _, err := io.Copy(out, rc); err != nil {
		return fmt.Errorf("failed to copy data to %s: %w", destPath, err)
	}

	return nil
}

// MoveToSubmitted copies the processed receipt to the submitted archive bucket inside a month folder (e.g. 2026-08/receipt.pdf) with submission metadata, then deletes it from the source processed bucket.
func (s *GCSStorageService) MoveToSubmitted(ctx context.Context, srcBucket, dstBucket, objectName, monthFolder, batchID string) error {
	dstObjectName := objectName
	if monthFolder != "" {
		base := filepath.Base(objectName)
		dstObjectName = fmt.Sprintf("%s/%s", strings.Trim(monthFolder, "/"), base)
	}

	srcObj := s.client.Bucket(srcBucket).Object(objectName)
	dstObj := s.client.Bucket(dstBucket).Object(dstObjectName)

	attrs, err := srcObj.Attrs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get attrs for gs://%s/%s: %w", srcBucket, objectName, err)
	}

	newMeta := make(map[string]string)
	for k, v := range attrs.Metadata {
		newMeta[k] = v
	}
	newMeta["status"] = "submitted"
	newMeta["submitted_at"] = time.Now().UTC().Format(time.RFC3339)
	newMeta["submission_batch_id"] = batchID

	copier := dstObj.CopierFrom(srcObj)
	copier.Metadata = newMeta
	copier.ContentType = attrs.ContentType

	if _, err := copier.Run(ctx); err != nil {
		return fmt.Errorf("failed to copy gs://%s/%s to gs://%s/%s: %w", srcBucket, objectName, dstBucket, dstObjectName, err)
	}

	// Delete from source processed bucket
	if err := srcObj.Delete(ctx); err != nil {
		return fmt.Errorf("copied to submitted archive but failed to delete from gs://%s/%s: %w", srcBucket, objectName, err)
	}

	return nil
}

// MoveToFailed copies an unmatchable or failed receipt to the failed coldline bucket inside a month folder if available, then deletes it from the source processed bucket.
func (s *GCSStorageService) MoveToFailed(ctx context.Context, srcBucket, dstBucket, objectName, monthFolder, reason string) error {
	dstObjectName := objectName
	if monthFolder != "" {
		base := filepath.Base(objectName)
		dstObjectName = fmt.Sprintf("%s/%s", strings.Trim(monthFolder, "/"), base)
	}

	srcObj := s.client.Bucket(srcBucket).Object(objectName)
	dstObj := s.client.Bucket(dstBucket).Object(dstObjectName)

	attrs, err := srcObj.Attrs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get attrs for gs://%s/%s: %w", srcBucket, objectName, err)
	}

	newMeta := make(map[string]string)
	for k, v := range attrs.Metadata {
		newMeta[k] = v
	}
	newMeta["status"] = "failed"
	newMeta["failed_at"] = time.Now().UTC().Format(time.RFC3339)
	newMeta["failure_reason"] = reason

	copier := dstObj.CopierFrom(srcObj)
	copier.Metadata = newMeta
	copier.ContentType = attrs.ContentType

	if _, err := copier.Run(ctx); err != nil {
		return fmt.Errorf("failed to copy gs://%s/%s to gs://%s/%s: %w", srcBucket, objectName, dstBucket, dstObjectName, err)
	}

	// Delete from source processed bucket
	if err := srcObj.Delete(ctx); err != nil {
		return fmt.Errorf("copied to failed bucket but failed to delete from gs://%s/%s: %w", srcBucket, objectName, err)
	}

	return nil
}

// DeleteObject deletes the specified object from a GCS bucket.
func (s *GCSStorageService) DeleteObject(ctx context.Context, bucket, objectName string) error {
	return s.client.Bucket(bucket).Object(objectName).Delete(ctx)
}
