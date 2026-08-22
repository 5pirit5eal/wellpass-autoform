#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

if [ -f .env ]; then
  # shellcheck source=/dev/null
  source "./.env"
fi

help() {
  cat <<'EOF'
Usage: ./Taskfile.sh <task> [args...]

Development Tasks:
  run                   Run the local CloudEvent functions server (.env)
  run-local             Run local server with .env.local
  build                 Build the Go server binary to bin/receipts-function
  test                  Run all unit and package tests
  format                Format Go source code (go fmt)
  validate              Run linting and static analysis (go vet, golangci-lint)
  clean                 Remove build artifacts

GCloud Authentication & Configuration:
  setup-gcloud          Set up and activate local gcloud configuration
  authenticate          Log in with Application Default Credentials (ADC)
  activate              Activate configuration for current PROJECT_ID

Deployment & Testing Tasks (Cloud Storage Trigger):
  deploy                Deploy 2nd-gen Cloud Run function triggered by GCS uploads
  describe [NAME]       Show details and status of deployed Cloud Run function
  logs [NAME]           Read recent execution logs for Cloud Run function
  upload-sample <file>  Upload a sample receipt to the source GCS bucket to trigger processing
  send-event <file> [URL] Send a simulated GCS CloudEvent to local server

Help:
  help                  Show this help text
EOF
}

run() {
  echo "Starting local receipts-function CloudEvent server..."
  go run ./cmd/server/main.go
}

run-local() {
  echo "Starting local receipts-function CloudEvent server with .env.local..."
  go run ./cmd/server/main.go -env .env.local
}

build() {
  echo "Building receipts-function binary..."
  mkdir -p bin
  go build -v -o bin/receipts-function ./cmd/server/main.go
  echo "Build complete: bin/receipts-function"
}

validate() {
  echo "Running go vet..."
  go vet ./...
  if command -v golangci-lint >/dev/null 2>&1; then
    echo "Running golangci-lint..."
    golangci-lint run --path-mode=abs
  else
    echo "golangci-lint not found; skipping."
  fi
  echo "Validation passed."
}

format() {
  echo "Formatting Go files..."
  go fmt ./...
}

test() {
  echo "Running tests with coverage..."
  go test -v -cover ./...
}

clean() {
  echo "Cleaning build artifacts..."
  rm -rf bin
}

authenticate() {
  gcloud auth login --update-adc --no-launch-browser
}

activate() {
  local project="${PROJECT_ID:?PROJECT_ID environment variable is required}"
  gcloud config configurations activate "$project"
  authenticate
  gcloud auth application-default set-quota-project "$project"
  echo "SUCCESS: GOOGLE CLOUD CONFIGURATION ACTIVATED ($project)"
}

setup-gcloud() {
  local project="${PROJECT_ID:?PROJECT_ID environment variable is required}"
  local region="${REGION:-europe-west3}"
  echo "--- SETTING UP LOCAL GOOGLE CLOUD SDK CONFIGURATION ---"
  gcloud config configurations create "$project" 2>/dev/null || true
  activate
  gcloud config set project "$project"
  gcloud config set compute/region "$region"
}

deploy() {
  local func_name="${FUNCTION_NAME:-process-receipt}"
  local project="${PROJECT_ID:-${GCP_PROJECT_ID:?PROJECT_ID is required}}"
  local region="${REGION:-${GCP_REGION:-europe-west3}}"
  local gemini_location="${GEMINI_LOCATION:-eu}"
  local source_bucket="${SOURCE_BUCKET:-${RECEIPTS_SOURCE_BUCKET:?SOURCE_BUCKET is required for storage trigger}}"
  local target_bucket="${TARGET_BUCKET:-${RECEIPTS_TARGET_BUCKET:?TARGET_BUCKET is required}}"
  local failed_bucket="${FAILED_BUCKET:-${RECEIPTS_FAILED_BUCKET:?FAILED_BUCKET is required}}"
  local gemini_model="${GEMINI_MODEL:-gemini-3.5-flash-lite}"
  local runtime="${RUNTIME:-go126}"
  local ar_repo="${ARTIFACT_REGISTRY_REPOSITORY:-golang}"
  local docker_repo_flag=()

  if [ -n "$ar_repo" ]; then
    local repo_path="projects/${project}/locations/${region}/repositories/${ar_repo}"
    docker_repo_flag=("--docker-repository=${repo_path}")
  fi

  echo "Deploying Cloud Storage triggered Cloud Run function: $func_name (trigger: gs://$source_bucket) to $region (Project: $project)..."
  gcloud functions deploy "$func_name" \
    --gen2 \
    --runtime "$runtime" \
    --region "$region" \
    --project "$project" \
    --source . \
    --entry-point ProcessReceipt \
    --trigger-bucket "$source_bucket" \
    "${docker_repo_flag[@]}" \
    --set-env-vars "SOURCE_BUCKET=${source_bucket},TARGET_BUCKET=${target_bucket},FAILED_BUCKET=${failed_bucket},GEMINI_MODEL=${gemini_model},REGION=${region},PROJECT_ID=${project},GEMINI_LOCATION=${gemini_location}" \
    --memory 512MB \
    --timeout 120s
}

describe() {
  local func_name="${1:-${FUNCTION_NAME:-process-receipt}}"
  local project="${PROJECT_ID:?PROJECT_ID is required}"
  local region="${REGION:-europe-west3}"
  gcloud functions describe "$func_name" --gen2 --region "$region" --project "$project"
}

logs() {
  local func_name="${1:-${FUNCTION_NAME:-process-receipt}}"
  local project="${PROJECT_ID:?PROJECT_ID is required}"
  local region="${REGION:-europe-west3}"
  gcloud functions logs read "$func_name" --gen2 --region "$region" --project "$project" --limit 50
}

upload-sample() {
  local file_path="${1:-}"
  local source_bucket="${SOURCE_BUCKET:?SOURCE_BUCKET is required}"

  if [ -z "$file_path" ]; then
    echo "Error: file path is required"
    echo "Usage: ./Taskfile.sh upload-sample <path-to-receipt.pdf>"
    exit 1
  fi

  if [ ! -f "$file_path" ]; then
    echo "Error: file not found: $file_path"
    exit 1
  fi

  local filename
  filename="$(basename "$file_path")"

  echo "Uploading $file_path to gs://${source_bucket}/${filename}..."
  gcloud storage cp "$file_path" "gs://${source_bucket}/${filename}"
  echo "Upload complete. Check logs with: ./Taskfile.sh logs"
}

send-event() {
  local object_name="${1:-}"
  local endpoint_url="${2:-http://localhost:8080/}"
  local source_bucket="${SOURCE_BUCKET:-my-source-bucket}"

  if [ -z "$object_name" ]; then
    echo "Error: object name is required"
    echo "Usage: ./Taskfile.sh send-event <object-name> [endpoint-url]"
    exit 1
  fi

  echo "Sending CloudEvent for gs://${source_bucket}/${object_name} to ${endpoint_url}..."
  curl -i -X POST "$endpoint_url" \
    -H "Content-Type: application/json" \
    -H "ce-specversion: 1.0" \
    -H "ce-type: google.cloud.storage.object.v1.finalized" \
    -H "ce-source: //storage.googleapis.com/projects/_/buckets/${source_bucket}" \
    -H "ce-id: test-event-1234" \
    -d "{
      \"bucket\": \"${source_bucket}\",
      \"name\": \"${object_name}\",
      \"contentType\": \"application/pdf\"
    }"
  echo
}

# Check if the provided argument matches any of the functions
if [ -n "${1:-}" ] && ! declare -f "$1" >/dev/null; then
  echo "Error: Unknown task '$1'"
  echo
  help
  exit 1
fi

# Run application if no argument is provided
"${@:-help}"
