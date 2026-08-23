package receipts

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/wellpass-autoform/receipts-function/internal/bigquery"
	"github.com/wellpass-autoform/receipts-function/internal/config"
	"github.com/wellpass-autoform/receipts-function/internal/extractor"
	"github.com/wellpass-autoform/receipts-function/internal/handler"
	"github.com/wellpass-autoform/receipts-function/internal/processor"
	"github.com/wellpass-autoform/receipts-function/internal/storage"
)

var (
	initOnce          sync.Once
	cloudEventHandler *handler.CloudEventHandler
	initErr           error
)

func init() {
	functions.CloudEvent("ProcessReceipt", ProcessReceipt)
}

func initializeDependencies(ctx context.Context) error {
	initOnce.Do(func() {
		cfg, err := config.LoadFromEnv()
		if err != nil {
			initErr = fmt.Errorf("failed to load config: %w", err)
			return
		}

		if err := cfg.Validate(); err != nil {
			log.Printf("Warning: configuration validation: %v", err)
		}

		store, err := storage.NewGCSStorage(ctx)
		if err != nil {
			initErr = fmt.Errorf("failed to create GCS client: %w", err)
			return
		}

		ext, err := extractor.NewGeminiExtractor(ctx, cfg)
		if err != nil {
			initErr = fmt.Errorf("failed to create Gemini extractor: %w", err)
			return
		}

		var recorder bigquery.Recorder
		if cfg.ProjectID != "" && cfg.BigQueryDataset != "" && cfg.BigQueryTable != "" {
			bqRec, bqErr := bigquery.NewBQRecorder(ctx, cfg)
			if bqErr != nil {
				log.Printf("Warning: could not initialize BigQuery recorder (%v); analytics will not be saved", bqErr)
			} else {
				recorder = bqRec
			}
		}

		proc := processor.NewReceiptProcessor(cfg, ext, store, recorder)
		cloudEventHandler = handler.NewCloudEventHandler(cfg, proc, store)
	})
	return initErr
}

// ProcessReceipt is the Cloud Function CloudEvent entrypoint triggered when a receipt is uploaded to Cloud Storage.
func ProcessReceipt(ctx context.Context, e cloudevents.Event) error {
	if err := initializeDependencies(ctx); err != nil {
		log.Printf("Initialization error: %v", err)
		return err
	}

	return cloudEventHandler.HandleCloudEvent(ctx, e)
}
