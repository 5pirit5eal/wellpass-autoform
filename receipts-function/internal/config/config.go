package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds runtime configuration for the receipts processing function.
type Config struct {
	// Bucket names (all configured via environment variables)
	SourceBucket   string `json:"source_bucket"`
	TargetBucket   string `json:"target_bucket"`
	FailedBucket   string `json:"failed_bucket"`
	GeminiLocation string `json:"gemini_location"`

	// Gemini configuration
	GeminiModel  string `json:"gemini_model"`
	ProjectID    string `json:"project_id"`
	Location     string `json:"location"`
	GeminiAPIKey string `json:"-"`
	UseVertexAI  bool   `json:"use_vertex_ai"`

	// BigQuery configuration (optional - if set, enables analytics recording)
	BigQueryDataset string `json:"bigquery_dataset"`
	BigQueryTable   string `json:"bigquery_table"`

	// Server configuration
	Port string `json:"port"`
}

// LoadFromEnv loads configuration from environment variables with fallbacks.
func LoadFromEnv() (*Config, error) {
	targetBucket := getFirstEnv("TARGET_BUCKET", "RECEIPTS_TARGET_BUCKET", "PROCESSED_BUCKET", "PROCESSED_RECEIPTS_BUCKET")
	failedBucket := getFirstEnv("FAILED_BUCKET", "RECEIPTS_FAILED_BUCKET", "FAILED_PROCESSING_BUCKET")
	sourceBucket := getFirstEnv("SOURCE_BUCKET", "RECEIPTS_SOURCE_BUCKET", "UNPROCESSED_BUCKET")

	geminiModel := getFirstEnv("GEMINI_MODEL")
	if geminiModel == "" {
		geminiModel = "gemini-3.5-flash-lite"
	}
	geminiLocation := getFirstEnv("GEMINI_LOCATION", "GOOGLE_CLOUD_LOCATION", "GCP_LOCATION")

	projectID := getFirstEnv("PROJECT_ID", "GOOGLE_CLOUD_PROJECT", "GCP_PROJECT")
	location := getFirstEnv("REGION", "LOCATION", "GOOGLE_CLOUD_REGION", "GOOGLE_CLOUD_LOCATION", "GCP_LOCATION")
	if location == "" {
		location = "europe-west3"
	}

	geminiAPIKey := getFirstEnv("GEMINI_API_KEY", "GOOGLE_API_KEY")

	// Determine if we should use Vertex AI
	useVertexAI := true
	if val := os.Getenv("GOOGLE_GENAI_USE_VERTEXAI"); val != "" {
		useVertexAI = val == "1" || strings.ToLower(val) == "true"
	} else if geminiAPIKey != "" && projectID == "" {
		useVertexAI = false
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	bqDataset := getFirstEnv("BIGQUERY_DATASET", "BIGQUERY_DATASET_ID")
	if bqDataset == "" {
		bqDataset = "receipts_processing"
	}
	bqTable := getFirstEnv("BIGQUERY_TABLE", "BIGQUERY_TABLE_ID")
	if bqTable == "" {
		bqTable = "processing_results"
	}

	cfg := &Config{
		TargetBucket:    targetBucket,
		FailedBucket:    failedBucket,
		SourceBucket:    sourceBucket,
		GeminiModel:     geminiModel,
		ProjectID:       projectID,
		Location:        location,
		GeminiLocation:  geminiLocation,
		GeminiAPIKey:    geminiAPIKey,
		UseVertexAI:     useVertexAI,
		BigQueryDataset: bqDataset,
		BigQueryTable:   bqTable,
		Port:            port,
	}

	return cfg, nil
}

// Validate ensures required configuration values are present.
func (c *Config) Validate() error {
	var missing []string
	if c.SourceBucket == "" {
		missing = append(missing, "SOURCE_BUCKET")
	}
	if c.TargetBucket == "" {
		missing = append(missing, "TARGET_BUCKET")
	}
	if c.FailedBucket == "" {
		missing = append(missing, "FAILED_BUCKET")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	return nil
}

func getFirstEnv(keys ...string) string {
	for _, key := range keys {
		if val := os.Getenv(key); val != "" {
			return strings.TrimSpace(val)
		}
	}
	return ""
}
