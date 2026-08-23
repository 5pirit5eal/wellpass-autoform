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
  run [args...]         Run the form submission job locally (default: dry-run)
  run-month <YYYY-MM>   Run the job locally for a specific month (e.g. 2026-08)
  build                 Build the Go job binary to bin/job
  test                  Run all unit and package tests
  format                Format Go source code (go fmt)
  validate              Run linting and static analysis (go vet, golangci-lint)
  clean                 Remove build artifacts
  docker-build          Build local Docker image
  docker-build-and-run  Build and execute the job inside local Docker container

GCloud Authentication & Configuration:
  setup-gcloud          Set up and activate local gcloud configuration
  authenticate          Log in with Application Default Credentials (ADC)
  activate              Activate configuration for current PROJECT_ID

Deployment & Execution Tasks (Cloud Run Job & Scheduler):
  deploy                Build image, deploy Cloud Run Job, and configure Cloud Scheduler
  deploy-job            Deploy/update only the Cloud Run Job
  deploy-scheduler      Deploy/update only the Cloud Scheduler trigger
  describe [NAME]       Show details and configuration of the deployed Cloud Run Job
  logs [NAME]           Read recent execution logs for the Cloud Run Job
  execute-job [NAME]    Execute the Cloud Run Job immediately and wait for completion
  trigger-scheduler     Manually trigger the Cloud Scheduler job

Help:
  help                  Show this help text
EOF
}

run() {
  echo "Executing form-submission-job locally..."
  go run ./cmd/job/main.go "$@"
}

run-month() {
  local month="${1:-}"
  if [ -z "$month" ]; then
    echo "Error: Month argument required (format: YYYY-MM, e.g. 2026-08)" >&2
    exit 1
  fi
  shift || true
  echo "Executing form-submission-job for month ${month}..."
  go run ./cmd/job/main.go --month="$month" "$@"
}

build() {
  echo "Building form-submission-job binary..."
  mkdir -p bin
  go build -v -o bin/job ./cmd/job/main.go
  echo "Build complete: bin/job"
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
  rm -rf bin /tmp/form-submission-screenshots
}

docker-build() {
  local image_name="${IMAGE_NAME:-wellpass-form-submission-job}"
  echo "Building Docker image ${image_name}:latest..."
  docker build -t "${image_name}:latest" .
}

docker-build-and-run() {
  local image_name="${IMAGE_NAME:-wellpass-form-submission-job}"
  docker-build

  echo "Running Docker container imitating Cloud Run Job environment..."
  local env_args=()
  if [ -f .env ]; then
    env_args+=("--env-file" ".env")
  fi

  # Mount gcloud Application Default Credentials if available
  local adc_mount=()
  local adc_path="${HOME}/.config/gcloud/application_default_credentials.json"
  if [ -f "$adc_path" ]; then
    adc_mount+=("-v" "${adc_path}:/root/.config/gcloud/application_default_credentials.json:ro")
    adc_mount+=("-e" "GOOGLE_APPLICATION_CREDENTIALS=/root/.config/gcloud/application_default_credentials.json")
  fi

  docker run --rm -it \
    "${env_args[@]}" \
    "${adc_mount[@]}" \
    "${image_name}:latest" "$@"
}

authenticate() {
  gcloud auth login --update-adc --no-launch-browser
}

activate() {
  local project="${PROJECT_ID:-${GCP_PROJECT_ID:?PROJECT_ID environment variable is required}}"
  gcloud config configurations activate "$project"
  authenticate
  gcloud auth application-default set-quota-project "$project"
  echo "SUCCESS: GOOGLE CLOUD CONFIGURATION ACTIVATED ($project)"
}

setup-gcloud() {
  local project="${PROJECT_ID:-${GCP_PROJECT_ID:?PROJECT_ID environment variable is required}}"
  local region="${REGION:-${GCP_REGION:-europe-west3}}"
  echo "--- SETTING UP LOCAL GOOGLE CLOUD SDK CONFIGURATION ---"
  gcloud config configurations create "$project" 2>/dev/null || true
  activate
  gcloud config set project "$project"
  gcloud config set compute/region "$region"
}

deploy-job() {
  local project="${PROJECT_ID:-${GCP_PROJECT_ID:?PROJECT_ID is required}}"
  local region="${REGION:-${GCP_REGION:-europe-west3}}"
  local repo_name="${ARTIFACT_REGISTRY_REPOSITORY:-${ARTIFACT_REGISTRY_REPO:-golang}}"
  local job_name="${JOB_NAME:-form-submission-job}"
  local source_bucket="${SOURCE_BUCKET:-${RECEIPTS_TARGET_BUCKET:?SOURCE_BUCKET or RECEIPTS_TARGET_BUCKET is required}}"
  local submitted_bucket="${SUBMITTED_BUCKET:-${RECEIPTS_SUBMITTED_BUCKET:-${source_bucket/processed/submitted}}}"
  local failed_bucket="${FAILED_BUCKET:-${RECEIPTS_FAILED_BUCKET:-${source_bucket/processed/failed}}}"
  local bq_dataset="${BIGQUERY_DATASET:-${BIGQUERY_DATASET_ID:-receipts_processing}}"
  local bq_table="${BIGQUERY_TABLE:-${BIGQUERY_TABLE_ID:-processing_results}}"
  local typeform_url="${TYPEFORM_URL:-${WELLPASS_TYPEFORM_URL:-https://egym.typeform.com/to/z5XBrNXf}}"
  local email="${EMAIL:-${WELLPASS_EMAIL:?EMAIL or WELLPASS_EMAIL is required}}"
  local first_name="${FIRST_NAME:-${WELLPASS_FIRST_NAME:?FIRST_NAME or WELLPASS_FIRST_NAME is required}}"
  local last_name="${LAST_NAME:-${WELLPASS_LAST_NAME:?LAST_NAME or WELLPASS_LAST_NAME is required}}"
  local iban="${IBAN:-${WELLPASS_IBAN:?IBAN or WELLPASS_IBAN is required}}"
  local bic="${BIC:-${WELLPASS_BIC:?BIC or WELLPASS_BIC is required}}"
  local dry_run="${DRY_RUN:-${SUBMISSION_DRY_RUN:-true}}"
  local job_sa="${SUBMISSION_JOB_SERVICE_ACCOUNT:-form-submission-job-sa@${project}.iam.gserviceaccount.com}"
  local image_tag="${region}-docker.pkg.dev/${project}/${repo_name}/${job_name}:latest"

  echo "==> Building container image via Google Cloud Build in ${region}: ${image_tag}..."
  gcloud builds submit . \
    --project="${project}" \
    --region="${region}" \
    --default-buckets-behavior=regional-user-owned-bucket \
    --tag="${image_tag}"

  echo "==> Deploying Cloud Run Job: ${job_name} in ${region}..."
  local env_vars="SOURCE_BUCKET=${source_bucket},SUBMITTED_BUCKET=${submitted_bucket},FAILED_BUCKET=${failed_bucket},PROJECT_ID=${project},BIGQUERY_DATASET=${bq_dataset},BIGQUERY_TABLE=${bq_table},TYPEFORM_URL=${typeform_url},EMAIL=${email},FIRST_NAME=${first_name},LAST_NAME=${last_name},IBAN=${iban},BIC=${bic},DRY_RUN=${dry_run},HEADLESS=true,SCREENSHOTS_DIR=/tmp/form-submission-screenshots"

  if gcloud run jobs describe "${job_name}" --project="${project}" --region="${region}" >/dev/null 2>&1; then
    gcloud run jobs update "${job_name}" \
      --project="${project}" \
      --region="${region}" \
      --image="${image_tag}" \
      --service-account="${job_sa}" \
      --memory="2Gi" \
      --cpu="1000m" \
      --max-retries=1 \
      --task-timeout="600s" \
      --set-env-vars="${env_vars}"
  else
    gcloud run jobs create "${job_name}" \
      --project="${project}" \
      --region="${region}" \
      --image="${image_tag}" \
      --service-account="${job_sa}" \
      --memory="2Gi" \
      --cpu="1000m" \
      --max-retries=1 \
      --task-timeout="600s" \
      --set-env-vars="${env_vars}"
  fi
  echo "Cloud Run Job ${job_name} deployed successfully."
}

deploy-scheduler() {
  local project="${PROJECT_ID:-${GCP_PROJECT_ID:?PROJECT_ID is required}}"
  local region="${REGION:-${GCP_REGION:-europe-west3}}"
  local job_name="${JOB_NAME:-form-submission-job}"
  local scheduler_name="${SCHEDULER_JOB_NAME:-monthly-form-submission-trigger}"
  local cron_schedule="${CRON_SCHEDULE:-0 8 2 * *}"
  local time_zone="${TIME_ZONE:-Europe/Berlin}"
  local scheduler_sa="${SCHEDULER_JOB_SERVICE_ACCOUNT:-scheduler-job-invoker-sa@${project}.iam.gserviceaccount.com}"
  local job_run_uri="https://${region}-run.googleapis.com/v2/projects/${project}/locations/${region}/jobs/${job_name}:run"

  echo "==> Deploying Cloud Scheduler Job: ${scheduler_name} (${cron_schedule})..."
  if gcloud scheduler jobs describe "${scheduler_name}" --project="${project}" --location="${region}" >/dev/null 2>&1; then
    gcloud scheduler jobs update http "${scheduler_name}" \
      --project="${project}" \
      --location="${region}" \
      --schedule="${cron_schedule}" \
      --time-zone="${time_zone}" \
      --uri="${job_run_uri}" \
      --http-method="POST" \
      --oauth-service-account-email="${scheduler_sa}"
  else
    gcloud scheduler jobs create http "${scheduler_name}" \
      --project="${project}" \
      --location="${region}" \
      --schedule="${cron_schedule}" \
      --time-zone="${time_zone}" \
      --description="Triggers monthly Wellpass reimbursement form submission Cloud Run Job" \
      --uri="${job_run_uri}" \
      --http-method="POST" \
      --oauth-service-account-email="${scheduler_sa}"
  fi
  echo "Cloud Scheduler trigger ${scheduler_name} deployed successfully."
}

deploy() {
  deploy-job
  deploy-scheduler
  echo "Full deployment completed successfully."
}

describe() {
  local job_name="${1:-${JOB_NAME:-form-submission-job}}"
  local project="${PROJECT_ID:-${GCP_PROJECT_ID:?PROJECT_ID is required}}"
  local region="${REGION:-${GCP_REGION:-europe-west3}}"
  gcloud run jobs describe "$job_name" --region "$region" --project "$project"
}

logs() {
  local job_name="${1:-${JOB_NAME:-form-submission-job}}"
  local project="${PROJECT_ID:-${GCP_PROJECT_ID:?PROJECT_ID is required}}"
  local region="${REGION:-${GCP_REGION:-europe-west3}}"
  echo "Fetching recent execution logs for Cloud Run Job ${job_name}..."
  gcloud logging read "resource.type=cloud_run_job AND resource.labels.job_name=${job_name}" --project "$project" --limit 50 --format="table(timestamp,severity,textPayload)"
}

execute-job() {
  local job_name="${1:-${JOB_NAME:-form-submission-job}}"
  local project="${PROJECT_ID:-${GCP_PROJECT_ID:?PROJECT_ID is required}}"
  local region="${REGION:-${GCP_REGION:-europe-west3}}"
  echo "Executing Cloud Run Job ${job_name} in ${region} (Project: ${project})..."
  gcloud run jobs execute "$job_name" --project "$project" --region "$region" --wait
}

trigger-scheduler() {
  local scheduler_name="${1:-${SCHEDULER_JOB_NAME:-monthly-form-submission-trigger}}"
  local project="${PROJECT_ID:-${GCP_PROJECT_ID:?PROJECT_ID is required}}"
  local region="${REGION:-${GCP_REGION:-europe-west3}}"
  echo "Manually triggering Cloud Scheduler job ${scheduler_name}..."
  gcloud scheduler jobs run "$scheduler_name" --project "$project" --location "$region"
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
