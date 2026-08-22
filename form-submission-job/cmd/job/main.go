package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/wellpass-autoform/form-submission-job/internal/config"
	"github.com/wellpass-autoform/form-submission-job/internal/matcher"
	"github.com/wellpass-autoform/form-submission-job/internal/processor"
	"github.com/wellpass-autoform/form-submission-job/internal/storage"
	"github.com/wellpass-autoform/form-submission-job/internal/submitter"
)

func main() {
	dryRunFlag := flag.Bool("dry-run", true, "Run in dry-run mode (do not submit final form)")
	monthFlag := flag.String("month", "", "Target month to submit receipts for (format: YYYY-MM, defaults to previous month)")
	headlessFlag := flag.Bool("headless", true, "Run browser in headless mode")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// CLI flags override environment variables if set explicitly
	if isFlagPassed("dry-run") {
		cfg.DryRun = *dryRunFlag
	}
	if isFlagPassed("month") {
		cfg.TargetMonth = *monthFlag
	}
	if isFlagPassed("headless") {
		cfg.Headless = *headlessFlag
	}

	ctx := context.Background()

	store, err := storage.NewGCSStorageService(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize GCS storage service: %v", err)
	}
	defer func() {
		_ = store.Close()
	}()

	poolMatcher := matcher.NewPoolMatcher(nil)
	formSubmitter := submitter.NewPlaywrightSubmitter()

	job := processor.NewJobProcessor(cfg, store, poolMatcher, formSubmitter, "")

	report, err := job.Run(ctx)
	if err != nil {
		log.Printf("Job completed with error: %v", err)
		if report != nil {
			printJSONReport(report)
		}
		os.Exit(1)
	}

	printJSONReport(report)
}

func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func printJSONReport(report *processor.RunReport) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Printf("Error serializing report: %v\n", err)
		return
	}
	fmt.Println(string(data))
}
