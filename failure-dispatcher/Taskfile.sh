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
  test                  Run all unit and package tests
  validate              Run linting and static analysis (go vet)
  format                Format Go source code (go fmt)
  clean                 Remove temporary artifacts

Deployment Tasks (Cloud Run Function via Pub/Sub):
  deploy                Deploy 2nd-gen Cloud Run function triggered by Pub/Sub topic
  describe [NAME]       Show details and status of deployed Cloud Run function
  logs [NAME]           Read recent execution logs for Cloud Run function

Help:
  help                  Show this help text
EOF
}

test() {
    echo "Running unit tests in failure-dispatcher..."
    go test -v -cover ./...
}

validate() {
    echo "Running go vet..."
    go vet ./...
}

format() {
    echo "Formatting Go files..."
    go fmt ./...
}

clean() {
    echo "Cleaning..."
    rm -f coverage.out
}

deploy() {
    local func_name="${FUNCTION_NAME:-failure-dispatcher}"
    local project="${PROJECT_ID:-${GCP_PROJECT_ID:?PROJECT_ID is required}}"
    local region="${REGION:-${GCP_REGION:-europe-west3}}"
    local topic="${PUBSUB_TOPIC:-cloud-run-job-failures}"
    local runtime="${RUNTIME:-go126}"
    local service_account="${FAILURE_DISPATCHER_SERVICE_ACCOUNT:-job-failure-dispatcher-sa@${project}.iam.gserviceaccount.com}"
    local owner="${GITHUB_OWNER:-5pirit5eal}"
    local repo="${GITHUB_REPOSITORY:-wellpass-autoform}"
    local secret_id="${ACTION_DISPATCH_SECRET_ID:-github-dispatch-token}"

    echo "Deploying failure-dispatcher Cloud Run function to ${region} (Trigger: Pub/Sub topic ${topic})..."
    gcloud functions deploy "${func_name}" \
        --gen2 \
        --runtime "${runtime}" \
        --region "${region}" \
        --project "${project}" \
        --source . \
        --entry-point DispatchFailure \
        --trigger-topic "${topic}" \
        --service-account "${service_account}" \
        --set-env-vars "PROJECT_ID=${project},GITHUB_OWNER=${owner},GITHUB_REPOSITORY=${repo},ACTION_DISPATCH_SECRET_ID=${secret_id},GITHUB_EVENT_TYPE=cloud-run-job-failure" \
        --memory 256MB \
        --timeout 60s
}

describe() {
    local func_name="${1:-${FUNCTION_NAME:-failure-dispatcher}}"
    local project="${PROJECT_ID:-${GCP_PROJECT_ID:?PROJECT_ID is required}}"
    local region="${REGION:-${GCP_REGION:-europe-west3}}"
    gcloud functions describe "$func_name" --gen2 --region "$region" --project "$project"
}

logs() {
    local func_name="${1:-${FUNCTION_NAME:-failure-dispatcher}}"
    local project="${PROJECT_ID:-${GCP_PROJECT_ID:?PROJECT_ID is required}}"
    local region="${REGION:-${GCP_REGION:-europe-west3}}"
    gcloud functions logs read "$func_name" --gen2 --region "$region" --project "$project" --limit 50
}

if [ -n "${1:-}" ] && ! declare -f "$1" >/dev/null; then
    echo "Error: Unknown task '$1'"
    echo
    help
    exit 1
fi

"${@:-help}"
