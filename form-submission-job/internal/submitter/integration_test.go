//go:build integration

package submitter

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPlaywrightLiveDryRun(t *testing.T) {
	// Locate sample receipt PDF in examples/
	projectRoot, err := filepath.Abs("../../../examples")
	if err != nil {
		t.Fatalf("failed to get project root: %v", err)
	}
	samplePDF := filepath.Join(projectRoot, "8e3e1eae307d427480b57565c1b89cdb.pdf")
	if _, err := os.Stat(samplePDF); err != nil {
		t.Skipf("sample receipt %s not found, skipping live integration test", samplePDF)
	}

	screenshotsDir := "/tmp/playwright-test-screenshots"
	_ = os.MkdirAll(screenshotsDir, 0755)

	batch := SubmissionBatch{
		BatchID:        "test_dry_run_batch",
		TypeformURL:    "https://egym.typeform.com/to/z5XBrNXf",
		Email:          "rubeneschulze@googlemail.com",
		FullName:       "Ruben Schulze",
		IBAN:           "DE98430609671319918600",
		BIC:            "GENODEM1GLS",
		DryRun:         true,
		Headless:       true,
		ScreenshotsDir: screenshotsDir,
		Tickets: []SubmissionTicket{
			{
				ObjectName: "8e3e1eae307d427480b57565c1b89cdb.pdf",
				PoolLabel:  "Schwimm' in Bilk Düsseldorf",
				Day:        "15",
				Month:      "08",
				Year:       "2026",
				Price:      "5,44",
				FilePath:   samplePDF,
			},
		},
	}

	submitter := NewPlaywrightSubmitter()
	res, err := submitter.Submit(context.Background(), batch)
	if err != nil {
		t.Fatalf("Playwright dry run submission failed: %v", err)
	}

	if !res.Success {
		t.Errorf("expected success true, got false")
	}
	if len(res.Screenshots) == 0 {
		t.Errorf("expected screenshots to be generated")
	}
	t.Logf("Generated %d screenshots: %v", len(res.Screenshots), res.Screenshots)
}
