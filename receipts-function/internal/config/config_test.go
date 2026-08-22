package config

import (
	"testing"
)

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("SOURCE_BUCKET", "my-source-bucket")
	t.Setenv("TARGET_BUCKET", "my-target-bucket")
	t.Setenv("FAILED_BUCKET", "my-failed-bucket")
	t.Setenv("PROJECT_ID", "my-gcp-project")
	t.Setenv("REGION", "europe-west3")
	t.Setenv("GEMINI_MODEL", "gemini-3.5-flash-lite")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.SourceBucket != "my-source-bucket" {
		t.Errorf("expected SourceBucket 'my-source-bucket', got '%s'", cfg.SourceBucket)
	}
	if cfg.TargetBucket != "my-target-bucket" {
		t.Errorf("expected TargetBucket 'my-target-bucket', got '%s'", cfg.TargetBucket)
	}
	if cfg.FailedBucket != "my-failed-bucket" {
		t.Errorf("expected FailedBucket 'my-failed-bucket', got '%s'", cfg.FailedBucket)
	}
	if cfg.ProjectID != "my-gcp-project" {
		t.Errorf("expected ProjectID 'my-gcp-project', got '%s'", cfg.ProjectID)
	}
	if cfg.Location != "europe-west3" {
		t.Errorf("expected Location 'europe-west3', got '%s'", cfg.Location)
	}
	if cfg.GeminiModel != "gemini-3.5-flash-lite" {
		t.Errorf("expected GeminiModel 'gemini-3.5-flash-lite', got '%s'", cfg.GeminiModel)
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("expected validation to pass, got: %v", err)
	}
}

func TestConfigValidationMissingBuckets(t *testing.T) {
	t.Setenv("SOURCE_BUCKET", "")
	t.Setenv("RECEIPTS_SOURCE_BUCKET", "")
	t.Setenv("TARGET_BUCKET", "")
	t.Setenv("RECEIPTS_TARGET_BUCKET", "")
	t.Setenv("PROCESSED_BUCKET", "")
	t.Setenv("PROCESSED_RECEIPTS_BUCKET", "")
	t.Setenv("FAILED_BUCKET", "")
	t.Setenv("RECEIPTS_FAILED_BUCKET", "")
	t.Setenv("FAILED_PROCESSING_BUCKET", "")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := cfg.Validate(); err == nil {
		t.Error("expected validation to fail when source, target and failed buckets are missing")
	}
}
