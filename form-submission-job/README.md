# Form Submission Job

Automated monthly batch submission job for EGYM Wellpass swimming pool ticket reimbursements via Typeform.

## Overview

The `form-submission-job` runs once per month (typically on the 2nd day of each month) to process and submit all approved swimming pool receipts from the previous calendar month.

### Features

- **GCS Integration**: Discovers processed receipts and extracts structured metadata attached by `receipts-function`.
- **Intelligent Pool Matching**: Embedded 423-pool catalog with fuzzy token, alias, and Levenshtein matching to resolve raw OCR locations (e.g., `"Schwimm in Bilk"`) to the exact Typeform dropdown label (`"Schwimm' in Bilk Düsseldorf"`).
- **Batching & Chunking**: Automatically slices receipts into batches of up to 10 tickets per submission run (matching Typeform limit).
- **Playwright Automation**: Full headless browser automation navigating the multi-step form, selecting pool dropdowns, inputting dates and prices, uploading ticket files, and filling banking details.
- **Dry-Run Safety**: Defaults to `DRY_RUN=true` to navigate all steps, upload files, and take visual screenshots without clicking the final submit button.
- **Audit Screenshots**: Saves PNG screenshots of each submission stage to `/tmp/form-submission-screenshots` for verification.

## Configuration

Set the following variables in `.env` or container environment:

| Variable | Description | Example |
| --- | --- | --- |
| `SOURCE_BUCKET` | GCS bucket containing processed receipts | `your-project-id-receipts-processed` |
| `SUBMITTED_BUCKET` | GCS archive bucket for submitted receipts | `your-project-id-receipts-submitted` |
| `FAILED_BUCKET` | GCS bucket for screenshots and unmatched items | `your-project-id-receipts-failed` |
| `PROJECT_ID` | Google Cloud Project ID | `your-project-id` |
| `BIGQUERY_DATASET` | BigQuery dataset ID | `receipts_processing` |
| `BIGQUERY_TABLE` | BigQuery table ID | `processing_results` |
| `TYPEFORM_URL` | EGYM Wellpass Typeform URL | `https://egym.typeform.com/to/z5XBrNXf` |
| `EMAIL` | EGYM Wellpass member email | `member@example.com` |
| `FIRST_NAME` | Member first name | `Max` |
| `LAST_NAME` | Member last name | `Mustermann` |
| `IBAN` | Member IBAN (without spaces) | `DE12345678901234567890` |
| `BIC` | Member BIC code | `GENODEM1GLS` |
| `DRY_RUN` | Prevent final submission click (`true`/`false`) | `true` |
| `HEADLESS` | Run browser in headless mode (`true`/`false`) | `true` |
| `TARGET_MONTH` | Target month (`YYYY-MM`, default: previous month) | `2026-08` |
| `SCREENSHOTS_DIR` | Directory for audit screenshots | `/tmp/form-submission-screenshots` |

---

## Project Structure

```
form-submission-job/
├── .env.example              # Example environment configuration
├── Dockerfile                # Multi-stage Playwright Go runtime container image
├── Taskfile.sh               # Automation script for local dev, test, and gcloud deployment
├── deploy.sh                 # Deployment script for Cloud Run Job & Cloud Scheduler
├── go.mod                    # Go module (1.26.3)
├── go.sum
├── cmd/
│   └── job/
│       └── main.go           # CLI job entrypoint
└── internal/
    ├── bigquery/             # BigQuery MERGE submission updater & types
    ├── config/               # Configuration loading and validation
    ├── matcher/              # 423-pool fuzzy token matching engine
    ├── processor/            # Batch orchestration, chunking, and GCS archiving
    ├── storage/              # GCS client, download, move, and metadata helpers
    └── submitter/            # Playwright browser automation against Typeform
```

---

## Local Development & Tasks

Use `./Taskfile.sh`:

```bash
# Run unit tests with coverage
./Taskfile.sh test

# Format and validate code
./Taskfile.sh validate

# Build binary
./Taskfile.sh build

# Execute dry-run for previous month
./Taskfile.sh run

# Execute dry-run for specific month
./Taskfile.sh run-month 2026-08
```

## Deployment via `gcloud` CLI

The Cloud Run Job and Cloud Scheduler trigger can be deployed and managed directly via `gcloud` commands or the helper script:

```bash
# 1. Full deployment (builds container image, creates/updates Cloud Run Job & Scheduler)
./Taskfile.sh deploy

# Or run deploy.sh directly:
./deploy.sh

# 2. Trigger an immediate execution of the Cloud Run Job:
./Taskfile.sh execute-job
# or via gcloud:
gcloud run jobs execute form-submission-job --region=europe-west3 --wait

# 3. Trigger the Cloud Scheduler trigger manually:
./Taskfile.sh trigger-scheduler
# or via gcloud:
gcloud scheduler jobs run monthly-form-submission-trigger --location=europe-west3
```

### Manual `gcloud` Commands

```bash
# Deploy Cloud Run Job
gcloud run jobs create form-submission-job \
  --region=europe-west3 \
  --image=europe-west3-docker.pkg.dev/$PROJECT_ID/golang/form-submission-job:latest \
  --service-account=form-submission-job-sa@$PROJECT_ID.iam.gserviceaccount.com \
  --memory=2Gi \
  --cpu=1000m \
  --task-timeout=3600s \
  --max-retries=1 \
  --set-env-vars="SOURCE_BUCKET=$PROJECT_ID-receipts-processed,SUBMITTED_BUCKET=$PROJECT_ID-receipts-submitted,FAILED_BUCKET=$PROJECT_ID-receipts-failed,PROJECT_ID=$PROJECT_ID,BIGQUERY_DATASET=receipts_processing,BIGQUERY_TABLE=processing_results,TYPEFORM_URL=https://egym.typeform.com/to/z5XBrNXf,EMAIL=$EMAIL,FIRST_NAME=$FIRST_NAME,LAST_NAME=$LAST_NAME,IBAN=$IBAN,BIC=$BIC,DRY_RUN=true,HEADLESS=true,SCREENSHOTS_DIR=/tmp/form-submission-screenshots"

# Deploy Cloud Scheduler Trigger
gcloud scheduler jobs create http monthly-form-submission-trigger \
  --location=europe-west3 \
  --schedule="0 8 2 * *" \
  --time-zone="Europe/Berlin" \
  --description="Triggers monthly Wellpass reimbursement form submission Cloud Run Job" \
  --uri="https://europe-west3-run.googleapis.com/v2/projects/$PROJECT_ID/locations/europe-west3/jobs/form-submission-job:run" \
  --http-method="POST" \
  --oauth-service-account-email="scheduler-job-invoker-sa@$PROJECT_ID.iam.gserviceaccount.com"
```
