package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MockStorageService is an in-memory mock for testing.
type MockStorageService struct {
	Items          []*ReceiptItem
	DownloadedData map[string][]byte
	SubmittedList  []string
	FailedList     []string
	DeletedList    []string
	UploadedFiles  map[string][]byte
}

// NewMockStorageService creates a new mock storage service.
func NewMockStorageService(items []*ReceiptItem) *MockStorageService {
	return &MockStorageService{
		Items:          items,
		DownloadedData: make(map[string][]byte),
		UploadedFiles:  make(map[string][]byte),
	}
}

func (m *MockStorageService) ListProcessedReceipts(ctx context.Context, bucket string, allowedMonths []string) ([]*ReceiptItem, error) {
	var result []*ReceiptItem
	allowedMap := make(map[string]bool)
	for _, mon := range allowedMonths {
		allowedMap[mon] = true
	}

	for _, item := range m.Items {
		if item.Status == "submitted" {
			continue
		}
		if len(allowedMap) > 0 && item.Date != "" {
			prefix := item.Date
			if len(item.Date) >= 7 {
				prefix = item.Date[:7]
			}
			if !allowedMap[prefix] {
				continue
			}
		}
		result = append(result, item)
	}
	return result, nil
}

func (m *MockStorageService) DownloadReceipt(ctx context.Context, bucket, objectName, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	content := []byte(fmt.Sprintf("%%PDF-1.4 mock content for %s", objectName))
	if custom, ok := m.DownloadedData[objectName]; ok {
		content = custom
	}
	return os.WriteFile(destPath, content, 0644)
}

func (m *MockStorageService) MoveToSubmitted(ctx context.Context, srcBucket, dstBucket, objectName, monthFolder, batchID string) error {
	dstObjName := objectName
	if monthFolder != "" {
		dstObjName = fmt.Sprintf("%s/%s", strings.Trim(monthFolder, "/"), filepath.Base(objectName))
	}
	m.SubmittedList = append(m.SubmittedList, dstObjName)
	m.DeletedList = append(m.DeletedList, objectName)
	for _, item := range m.Items {
		if item.ObjectName == objectName {
			item.Bucket = dstBucket
			item.ObjectName = dstObjName
			item.Status = "submitted"
			item.SubmittedAt = time.Now().UTC().Format(time.RFC3339)
		}
	}
	return nil
}

func (m *MockStorageService) MoveToFailed(ctx context.Context, srcBucket, dstBucket, objectName, monthFolder, reason string) error {
	dstObjName := objectName
	if monthFolder != "" {
		dstObjName = fmt.Sprintf("%s/%s", strings.Trim(monthFolder, "/"), filepath.Base(objectName))
	}
	m.FailedList = append(m.FailedList, dstObjName)
	m.DeletedList = append(m.DeletedList, objectName)
	for _, item := range m.Items {
		if item.ObjectName == objectName {
			item.Bucket = dstBucket
			item.ObjectName = dstObjName
			item.Status = "failed"
		}
	}
	return nil
}

func (m *MockStorageService) DeleteObject(ctx context.Context, bucket, objectName string) error {
	m.DeletedList = append(m.DeletedList, objectName)
	return nil
}

func (m *MockStorageService) UploadFile(ctx context.Context, bucket, objectName, localFilePath, contentType string, metadata map[string]string) error {
	data, err := os.ReadFile(localFilePath)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("%s/%s", bucket, objectName)
	m.UploadedFiles[key] = data
	return nil
}
