package config

import (
	"os"
	"testing"
)

func TestConfigLoad(t *testing.T) {
	// Setup test environment
	_ = os.Setenv("SOURCE_BUCKET", "test-bucket")
	_ = os.Setenv("EMAIL", "test@example.com")
	_ = os.Setenv("FIRST_NAME", "Max")
	_ = os.Setenv("LAST_NAME", "Mustermann")
	_ = os.Setenv("IBAN", "DE12 3456 7890")
	_ = os.Setenv("BIC", "GENODEM1GLS")
	_ = os.Setenv("DRY_RUN", "false")
	_ = os.Setenv("HEADLESS", "true")
	_ = os.Setenv("TARGET_MONTH", "2026-05")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}

	if cfg.SourceBucket != "test-bucket" {
		t.Errorf("got SourceBucket %q, want %q", cfg.SourceBucket, "test-bucket")
	}
	if cfg.Email != "test@example.com" {
		t.Errorf("got Email %q, want %q", cfg.Email, "test@example.com")
	}
	if cfg.FullName() != "Max Mustermann" {
		t.Errorf("got FullName %q, want %q", cfg.FullName(), "Max Mustermann")
	}
	if cfg.IBAN != "DE1234567890" {
		t.Errorf("got IBAN %q, want %q", cfg.IBAN, "DE1234567890")
	}
	if cfg.DryRun != false {
		t.Errorf("got DryRun %v, want false", cfg.DryRun)
	}
	if cfg.TargetSubmissionMonth() != "2026-05" {
		t.Errorf("got TargetSubmissionMonth %q, want %q", cfg.TargetSubmissionMonth(), "2026-05")
	}
}

func TestConfigValidation(t *testing.T) {
	os.Clearenv()

	_, err := Load()
	if err == nil {
		t.Fatal("expected error on empty env, got nil")
	}
}
