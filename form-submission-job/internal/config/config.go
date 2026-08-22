package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config contains all runtime settings for the form submission job.
type Config struct {
	SourceBucket    string
	SubmittedBucket string
	FailedBucket    string
	TypeformURL     string
	Email           string
	FirstName       string
	LastName        string
	IBAN            string
	BIC             string
	DryRun          bool
	Headless        bool
	TargetMonth     string // e.g. "2026-07" or "" for previous month
	ScreenshotsDir  string
}

// FullName returns first and last name concatenated.
func (c *Config) FullName() string {
	return strings.TrimSpace(fmt.Sprintf("%s %s", c.FirstName, c.LastName))
}

// TargetSubmissionMonth returns the YYYY-MM string to filter receipts for.
// Defaults to the previous month in UTC if TargetMonth is not explicitly configured.
func (c *Config) TargetSubmissionMonth() string {
	if c.TargetMonth != "" {
		return c.TargetMonth
	}
	now := time.Now().UTC()
	prevMonth := now.AddDate(0, -1, 0)
	return prevMonth.Format("2006-01")
}

// Load reads configuration from environment variables and an optional .env file.
func Load() (*Config, error) {
	// Try loading .env if present
	_ = godotenv.Load()

	sourceBucket := strings.TrimSpace(os.Getenv("SOURCE_BUCKET"))
	if sourceBucket == "" {
		return nil, fmt.Errorf("SOURCE_BUCKET environment variable is required")
	}

	submittedBucket := strings.TrimSpace(os.Getenv("SUBMITTED_BUCKET"))
	if submittedBucket == "" {
		// Fallback: replace "processed" with "submitted"
		submittedBucket = strings.Replace(sourceBucket, "processed", "submitted", 1)
	}

	failedBucket := strings.TrimSpace(os.Getenv("FAILED_BUCKET"))
	if failedBucket == "" {
		// Fallback: replace "processed" with "failed"
		failedBucket = strings.Replace(sourceBucket, "processed", "failed", 1)
	}

	typeformURL := strings.TrimSpace(os.Getenv("TYPEFORM_URL"))
	if typeformURL == "" {
		typeformURL = "https://egym.typeform.com/to/z5XBrNXf"
	}

	email := strings.TrimSpace(os.Getenv("EMAIL"))
	if email == "" {
		return nil, fmt.Errorf("EMAIL environment variable is required")
	}

	firstName := strings.TrimSpace(os.Getenv("FIRST_NAME"))
	lastName := strings.TrimSpace(os.Getenv("LAST_NAME"))
	if firstName == "" || lastName == "" {
		return nil, fmt.Errorf("FIRST_NAME and LAST_NAME environment variables are required")
	}

	iban := strings.ReplaceAll(strings.TrimSpace(os.Getenv("IBAN")), " ", "")
	if iban == "" {
		return nil, fmt.Errorf("IBAN environment variable is required")
	}

	bic := strings.ReplaceAll(strings.TrimSpace(os.Getenv("BIC")), " ", "")
	if bic == "" {
		return nil, fmt.Errorf("BIC environment variable is required")
	}

	dryRun := true
	if dryRunStr := os.Getenv("DRY_RUN"); dryRunStr != "" {
		if parsed, err := strconv.ParseBool(dryRunStr); err == nil {
			dryRun = parsed
		}
	}

	headless := true
	if headlessStr := os.Getenv("HEADLESS"); headlessStr != "" {
		if parsed, err := strconv.ParseBool(headlessStr); err == nil {
			headless = parsed
		}
	}

	screenshotsDir := strings.TrimSpace(os.Getenv("SCREENSHOTS_DIR"))
	if screenshotsDir == "" {
		screenshotsDir = "/tmp/form-submission-screenshots"
	}

	targetMonth := strings.TrimSpace(os.Getenv("TARGET_MONTH"))

	return &Config{
		SourceBucket:    sourceBucket,
		SubmittedBucket: submittedBucket,
		FailedBucket:    failedBucket,
		TypeformURL:     typeformURL,
		Email:           email,
		FirstName:       firstName,
		LastName:        lastName,
		IBAN:            iban,
		BIC:             bic,
		DryRun:          dryRun,
		Headless:        headless,
		TargetMonth:     targetMonth,
		ScreenshotsDir:  screenshotsDir,
	}, nil
}
