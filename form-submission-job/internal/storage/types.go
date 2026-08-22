package storage

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ReceiptItem represents a processed receipt stored in GCS with metadata.
type ReceiptItem struct {
	Bucket        string    `json:"bucket"`
	ObjectName    string    `json:"object_name"`
	Date          string    `json:"date"` // YYYY-MM-DD
	TicketPrice   float64   `json:"ticket_price"`
	Currency      string    `json:"currency"`
	Location      string    `json:"location"`
	ReceiptNumber string    `json:"receipt_number,omitempty"`
	CustomerName  string    `json:"customer_name,omitempty"`
	Status        string    `json:"status"`
	SubmittedAt   string    `json:"submitted_at,omitempty"`
	LocalFilePath string    `json:"local_file_path,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// FormatPriceGerman formats a price as German currency string (e.g. 5.44 -> "5,44").
func (r *ReceiptItem) FormatPriceGerman() string {
	return strings.ReplaceAll(fmt.Sprintf("%.2f", r.TicketPrice), ".", ",")
}

// SplitDate returns (day, month, year) strings with leading zeros (e.g. "15", "08", "2026").
func (r *ReceiptItem) SplitDate() (day, month, year string, err error) {
	parts := strings.Split(r.Date, "-")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid date format %q (expected YYYY-MM-DD)", r.Date)
	}
	year = parts[0]
	month = fmt.Sprintf("%02s", parts[1])
	day = fmt.Sprintf("%02s", parts[2])
	return day, month, year, nil
}

// ParseMetadata populates a ReceiptItem from GCS custom metadata map.
func ParseMetadata(bucket, objectName string, meta map[string]string, created time.Time) *ReceiptItem {
	item := &ReceiptItem{
		Bucket:        bucket,
		ObjectName:    objectName,
		Date:          meta["date"],
		Currency:      meta["currency"],
		Location:      meta["location"],
		ReceiptNumber: meta["receipt_number"],
		CustomerName:  meta["customer_name"],
		Status:        meta["status"],
		SubmittedAt:   meta["submitted_at"],
		CreatedAt:     created,
	}

	if item.Currency == "" {
		item.Currency = "EUR"
	}

	if priceStr := meta["ticket_price"]; priceStr != "" {
		if p, err := strconv.ParseFloat(priceStr, 64); err == nil {
			item.TicketPrice = p
		}
	}

	return item
}

// StorageService defines the contract for interacting with receipt storage in GCS.
type StorageService interface {
	ListProcessedReceipts(ctx context.Context, bucket string, allowedMonths []string) ([]*ReceiptItem, error)
	DownloadReceipt(ctx context.Context, bucket, objectName, destPath string) error
	MoveToSubmitted(ctx context.Context, srcBucket, dstBucket, objectName, monthFolder, batchID string) error
	MoveToFailed(ctx context.Context, srcBucket, dstBucket, objectName, monthFolder, reason string) error
	DeleteObject(ctx context.Context, bucket, objectName string) error
	UploadFile(ctx context.Context, bucket, objectName, localFilePath, contentType string, metadata map[string]string) error
}
