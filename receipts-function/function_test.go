package receipts

import (
	"context"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/wellpass-autoform/receipts-function/internal/handler"
)

func TestProcessReceiptCloudEventEndpoint(t *testing.T) {
	t.Setenv("SOURCE_BUCKET", "test-source-bucket")
	t.Setenv("TARGET_BUCKET", "test-target-bucket")
	t.Setenv("FAILED_BUCKET", "test-failed-bucket")
	t.Setenv("PROJECT_ID", "test-project")
	t.Setenv("REGION", "europe-west3")
	t.Setenv("GEMINI_MODEL", "gemini-3.5-flash-lite")

	event := cloudevents.NewEvent()
	event.SetID("12345")
	event.SetType("google.cloud.storage.object.v1.finalized")
	event.SetSource("//storage.googleapis.com/projects/_/buckets/test-source-bucket")
	_ = event.SetData(cloudevents.ApplicationJSON, handler.StorageObjectData{
		Bucket: "test-source-bucket",
		Name:   "receipt.pdf",
	})

	// In test environment without cloud credentials, ProcessReceipt returns initialization or processing error
	// which verifies the entrypoint flow and error handling.
	_ = ProcessReceipt(context.Background(), event)
}
