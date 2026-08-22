# Receipts Processing Cloud Run Function

A Google Cloud Run Function (implemented in Go 1.26) triggered automatically when swimming pool tickets or receipts (PDF or TXT) are uploaded to a Cloud Storage bucket. The function processes them with **Gemini 3.5 Flash Lite** (`gemini-3.5-flash-lite`).

The function extracts structured receipt metadata, verifies there are no duplicate/conflicting objects in the target Cloud Storage bucket, and attaches the structured data as object metadata during upload. If conflicts or parsing failures occur, the receipt is stored in the failed processing bucket with detailed diagnostics.

---

## Key Features

- **Cloud Storage Trigger**: Automatically invoked whenever a new file is uploaded to `SOURCE_BUCKET` (`google.cloud.storage.object.v1.finalized` CloudEvent).
- **Gemini 3.5 Flash Lite Integration**: Uses `google.golang.org/genai` with structured JSON schema output to reliably extract:
  - **Date**: ISO 8601 format (`YYYY-MM-DD`, e.g. `2026-08-07`)
  - **Ticket Price**: Numeric ticket price in EUR (e.g. `5.44`)
  - **Location**: Specific swimming pool or facility (e.g. `Schwimm in Bilk`, `Freibad Allwetterbad Flingern`)
  - **Receipt Number**: Invoice, booking, or reservation number (e.g. `R-1091126`, `104307524/1`)
  - **Currency**, **Customer Name**, and **Ticket Type**
- **Conflict Prevention**: Prior to uploading to the target bucket, checks whether the object already exists in `TARGET_BUCKET`. Conflicting items are safely routed to the `FAILED_BUCKET` with conflict reasons attached.
- **Metadata Attachment**: Uploads the raw PDF/TXT receipt to `TARGET_BUCKET` with custom metadata key-value pairs (`date`, `ticket_price`, `currency`, `location`, `receipt_number`, `status`).
- **Unprocessed Bucket Cleanup**: Once a document is successfully processed into `TARGET_BUCKET` or moved into `FAILED_BUCKET`, it is automatically deleted from `SOURCE_BUCKET`.
- **Loop Prevention**: Automatically ignores events originating from `TARGET_BUCKET` or `FAILED_BUCKET`.
- **Environment-Driven Configuration**: All bucket names and parameters are determined by environment variables.

---

## Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `SOURCE_BUCKET` | **Yes** | — | GCS bucket where incoming receipts are uploaded to trigger the function |
| `TARGET_BUCKET` | **Yes** | — | GCS bucket for successfully processed receipts with attached metadata |
| `FAILED_BUCKET` | **Yes** | — | GCS bucket for conflicting or failed receipts |
| `PROJECT_ID` / `GOOGLE_CLOUD_PROJECT` | **Yes** (Vertex AI) | — | Google Cloud Project ID |
| `REGION` / `LOCATION` | No | `europe-west3` | Google Cloud region |
| `ARTIFACT_REGISTRY_REPOSITORY` | No | `golang` | Artifact Registry Docker repository name for function images |
| `GEMINI_MODEL` | No | `gemini-3.5-flash-lite` | Gemini model name |
| `GOOGLE_GENAI_USE_VERTEXAI` | No | `true` | Use Vertex AI backend with Application Default Credentials |
| `GEMINI_API_KEY` | No | — | Optional API key if using Google AI Studio instead of Vertex AI |
| `PORT` | No | `8080` | Local server port |

---

## Project Structure

```
receipts-function/
├── .env.example              # Example environment configuration
├── Taskfile.sh               # Automation script for local dev, test, and gcloud deployment
├── function.go               # Cloud Run Function CloudEvent entrypoint (ProcessReceipt)
├── function_test.go          # CloudEvent entrypoint tests
├── go.mod                    # Go module (1.26.3)
├── go.sum
├── cmd/
│   └── server/
│       └── main.go           # Local CloudEvent development server
└── internal/
    ├── config/               # Configuration management and validation
    ├── extractor/            # Gemini 3.5 Flash Lite receipt extraction & schema
    ├── handler/              # CloudEvent request handler & loop prevention
    ├── processor/            # Processing workflow and conflict detection
    └── storage/              # Google Cloud Storage operations & metadata handling
```

---

## Local Development & Testing

### 1. Set Up Environment

Copy the example `.env` file:
```bash
cp .env.example .env
# Edit .env with your SOURCE_BUCKET, TARGET_BUCKET, FAILED_BUCKET, and PROJECT_ID
```

### 2. Available Tasks (`Taskfile.sh`)

```bash
# Display help and all available tasks
./Taskfile.sh help

# Format source code
./Taskfile.sh format

# Run linter and static analysis (go vet and golangci-lint)
./Taskfile.sh validate

# Run unit tests with coverage
./Taskfile.sh test

# Build local server binary
./Taskfile.sh build

# Start local CloudEvent server on port 8080 (loads .env)
./Taskfile.sh run
```

### 3. Test Locally with Simulated CloudEvent

With the local server running (`./Taskfile.sh run`), send a simulated CloudEvent:

```bash
./Taskfile.sh send-event sample-receipt.pdf http://localhost:8080/
```

---

## Deployment to Google Cloud Run Functions

The deployment is automated using `gcloud` CLI commands via `Taskfile.sh`.

### Prerequisites

1. Set up and authenticate `gcloud`:
   ```bash
   ./Taskfile.sh setup-gcloud
   # or
   ./Taskfile.sh authenticate
   ```
2. Enable the required GCP services:
   ```bash
   gcloud services enable \
     run.googleapis.com \
     cloudfunctions.googleapis.com \
     eventarc.googleapis.com \
     artifactregistry.googleapis.com \
     cloudbuild.googleapis.com \
     aiplatform.googleapis.com \
     storage.googleapis.com
   ```

### Deploying the Function

Deploy the function triggered by Cloud Storage uploads:

```bash
./Taskfile.sh deploy
```

Equivalent direct `gcloud` command:
```bash
gcloud functions deploy process-receipt \
  --gen2 \
  --runtime go126 \
  --region europe-west3 \
  --project $PROJECT_ID \
  --source . \
  --entry-point ProcessReceipt \
  --trigger-bucket "$SOURCE_BUCKET" \
  --docker-repository "projects/${PROJECT_ID}/locations/europe-west3/repositories/golang" \
  --set-env-vars "SOURCE_BUCKET=${SOURCE_BUCKET},TARGET_BUCKET=${TARGET_BUCKET},FAILED_BUCKET=${FAILED_BUCKET},GEMINI_MODEL=gemini-3.5-flash-lite,REGION=europe-west3,PROJECT_ID=${PROJECT_ID}" \
  --memory 512MB \
  --timeout 120s
```

### Testing the Deployed Function

Upload a receipt from `examples/` to the source bucket:

```bash
./Taskfile.sh upload-sample ../examples/8e3e1eae307d427480b57565c1b89cdb.pdf
```

Check the logs to verify processing:

```bash
./Taskfile.sh logs process-receipt
```
